// Package externalsort sorts fixed-size records in lexicographic byte order
// using bounded chunks encrypted with AES-256-GCM in owner-only temporary storage.
// Start with NewFactory and Factory.Open, add records, iterate once with
// Store.ForEachSorted, and close every non-nil Store, even after a failed Open.
// Callback records are borrowed plaintext and must be copied before retention.
//
// The package owns per-store storage and cleanup, not record encoding, key
// derivation, atomic output publication or crash recovery. Callers must protect
// process memory, isolate a trusted parent directory and coordinate shutdown.
// Context cancellation is cooperative; it cannot interrupt synchronous IO,
// entropy reads, sorting or a blocked callback. No goroutines, external service
// or cgo are introduced. Factory.Open refuses js and plan9 because rooted
// filesystem containment is unavailable there.
package externalsort

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	// AES256KeyBytes is the exact key size accepted by Factory.Open.
	AES256KeyBytes = 32
	// MaximumMergeFiles is the hard upper bound for one merge fan-in.
	MaximumMergeFiles = 64
	// MaximumRecordBytes bounds one fixed-size record.
	MaximumRecordBytes = 1 << 20
	// MaximumChunkRecords bounds the number of records held in memory.
	MaximumChunkRecords = 1_000_000
	// MaximumChunkBytes bounds the contiguous in-memory sort buffer.
	MaximumChunkBytes = 256 << 20

	storeIdentityBytes = 32
	temporaryNameBytes = 16
	temporaryAttempts  = 128
	nonceCounterOffset = 8
	chunkPrefix        = "chunk-"
	storePrefix        = ".external-sort-"
	aadVersion         = "extsort1"
	redacted           = "[REDACTED]"
	redactedJSON       = `"` + redacted + `"`
)

var (
	// ErrInvalidConfiguration reports invalid bounds, nil required inputs or an
	// unconstructed Factory/Store.
	ErrInvalidConfiguration = errors.New(
		"external sort configuration is invalid",
	)
	// ErrUnsafeParent reports a missing, replaced or insufficiently restricted
	// parent directory, or a platform without supported rooted containment.
	ErrUnsafeParent = errors.New(
		"external sort parent is not an owner-only directory",
	)
	// ErrInvalidKey reports a key whose length is not AES256KeyBytes.
	ErrInvalidKey = errors.New("external sort key is invalid")
	// ErrInvalidRecord reports a record whose length differs from Config.RecordBytes.
	ErrInvalidRecord = errors.New("external sort record is invalid")
	// ErrRecordLimit reports an Add beyond Config.MaximumRecords.
	ErrRecordLimit = errors.New("external sort record limit reached")
	// ErrConcurrentUse reports overlap with an active store lifecycle operation.
	ErrConcurrentUse = errors.New("external sort store is already in use")
	// ErrClosed reports input or iteration attempted after closing begins,
	// including cleanup-only handles returned by a failed Open.
	ErrClosed = errors.New("external sort store is closed")
	// ErrFinalized reports input or iteration attempted after finalization.
	ErrFinalized = errors.New("external sort store is finalized")
	// ErrEntropy reports failed randomness or exhausted nonce uniqueness state.
	ErrEntropy = errors.New("external sort entropy failed")
	// ErrStorage reports owned IO or cleanup failure without exposing paths
	// or wrapping underlying filesystem errors.
	ErrStorage = errors.New("external sort storage failed")
	// ErrCorrupt reports invalid encrypted framing or authentication,
	// including unexpected truncation or trailing bytes.
	ErrCorrupt          = errors.New("external sort chunk is corrupt")
	processNonceDomains = nonceDomainAllocator{entropy: rand.Reader}
)

// Config declares per-store storage and buffer bounds. There are no defaults;
// the zero value is invalid. These bounds do not limit process-wide memory,
// caller copies, cryptographic scratch or the number of concurrent stores.
type Config struct {
	// ParentDirectory is an existing absolute directory with no group/other
	// permission bits. Its final component must not be a symlink; ancestor
	// links are resolved at construction. The caller must exclude hostile writers.
	ParentDirectory string
	// RecordBytes is the exact positive record width, at most MaximumRecordBytes.
	RecordBytes int
	// ChunkRecords is the positive in-memory record capacity, at most
	// MaximumChunkRecords. RecordBytes * ChunkRecords must fit MaximumChunkBytes.
	ChunkRecords int
	// MaximumRecords is the total input ceiling, at least ChunkRecords.
	// The rounded-up chunk count must not exceed MaximumMergeFiles.
	MaximumRecords int
}

// Factory holds a validated reusable policy and parent-directory identity.
// Obtain it from NewFactory; its zero value cannot Open a store. It is safe for
// concurrent Open calls, which create independent stores. The factory owns no
// open directory handle or Close obligation; every store owns its own resources.
type Factory struct {
	config          Config
	parentInfo      os.FileInfo
	entropy         io.Reader
	nameEntropy     io.Reader
	openRoot        func(string) (rootDirectory, error)
	mkdir           func(rootDirectory, string, os.FileMode) error
	chmod           func(rootDirectory, string, os.FileMode) error
	removeAll       func(rootDirectory, string) error
	nextNonceDomain func() (uint64, bool)
}

// String prevents configuration and temporary paths from entering text logs.
func (Factory) String() string {
	return redacted
}

// GoString prevents configuration and temporary paths from entering debug
// representations.
func (Factory) GoString() string {
	return redacted
}

// LogValue prevents configuration and temporary paths from entering slog.
func (Factory) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON prevents configuration and temporary paths from entering JSON.
func (Factory) MarshalJSON() ([]byte, error) {
	return []byte(redactedJSON), nil
}

// NewFactory copies config after validating its bounds and existing absolute
// parent directory. The final parent component must not be a symlink and its
// group/other permission bits must be clear. Existing ancestor links are resolved
// once; Open revalidates the resolved directory's identity and permissions.
// Invalid bounds return nil and ErrInvalidConfiguration; filesystem/parent
// validation failures return nil and ErrUnsafeParent without revealing paths.
// No OS-owner or hostile-writer isolation check is performed.
func NewFactory(config Config) (*Factory, error) {
	if !validConfig(config) {
		return nil, ErrInvalidConfiguration
	}
	configuredParent := filepath.Clean(config.ParentDirectory)
	resolvedParent, err := filepath.EvalSymlinks(configuredParent)
	if err != nil {
		return nil, ErrUnsafeParent
	}
	parentInfo, err := validatedParent(configuredParent)
	if err != nil {
		return nil, err
	}
	config.ParentDirectory = resolvedParent

	return &Factory{
		config:      config,
		parentInfo:  parentInfo,
		entropy:     rand.Reader,
		nameEntropy: rand.Reader,
		openRoot:    openRootDirectory,
		mkdir: func(root rootDirectory, name string, mode os.FileMode) error {
			return root.Mkdir(name, mode)
		},
		chmod: func(root rootDirectory, name string, mode os.FileMode) error {
			return root.Chmod(name, mode)
		},
		removeAll: func(root rootDirectory, name string) error {
			return root.RemoveAll(name)
		},
		nextNonceDomain: allocateNonceDomain,
	}, nil
}

// Open creates an independent store with a rooted parent handle, owner-only
// work directory and bounded plaintext buffer. ctx must be non-nil and key must
// have AES256KeyBytes bytes; nil/zero factories return ErrInvalidConfiguration.
// Key bytes are consumed during cipher construction, not retained as the input
// slice. Do not mutate key during Open. The caller owns its lifetime; cipher
// schedules are not securely erased. Use an independently derived key per
// dataset across processes/restarts.
//
// Open checks cancellation cooperatively and returns context errors directly.
// Parent validation, entropy and storage failures use ErrUnsafeParent,
// ErrEntropy and ErrStorage. On js/plan9, rooted storage returns ErrUnsafeParent.
// These errors do not wrap sensitive underlying IO values.
//
// The caller must Close every non-nil Store, including one returned with an
// error: failed construction cleanup can leave an owned cleanup-only handle.
// Such a handle rejects Add/ForEachSorted with ErrClosed and retries removal
// through Close. A nil Store transfers no resource ownership.
func (factory *Factory) Open(
	ctx context.Context,
	key []byte,
) (*Store, error) {
	if factory == nil || !validConfig(factory.config) ||
		factory.parentInfo == nil || factory.entropy == nil ||
		factory.nameEntropy == nil ||
		factory.openRoot == nil || factory.mkdir == nil ||
		factory.chmod == nil || factory.removeAll == nil ||
		factory.nextNonceDomain == nil {
		return nil, ErrInvalidConfiguration
	}
	if ctx == nil {
		return nil, ErrInvalidConfiguration
	}
	if len(key) != AES256KeyBytes {
		return nil, ErrInvalidKey
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	block, _ := aes.NewCipher(key)
	authenticatedCipher, _ := cipher.NewGCM(block)
	identity := make([]byte, storeIdentityBytes)
	if _, err := io.ReadFull(factory.entropy, identity); err != nil {
		clear(identity)

		return nil, ErrEntropy
	}
	nonceDomain, ok := factory.nextNonceDomain()
	if !ok {
		clear(identity)

		return nil, ErrEntropy
	}
	root, err := factory.openRoot(factory.config.ParentDirectory)
	if err != nil {
		clear(identity)

		return nil, ErrUnsafeParent
	}
	parentInfo, err := root.Stat(".")
	if err != nil || !sameSafeDirectory(factory.parentInfo, parentInfo) {
		_ = root.Close()
		clear(identity)

		return nil, ErrUnsafeParent
	}
	directoryName, cleanupRequired, err := createStoreDirectory(
		ctx,
		root,
		factory.nameEntropy,
		factory.mkdir,
		factory.chmod,
		factory.removeAll,
	)
	if err != nil {
		if cleanupRequired {
			clear(identity)
			directory := filepath.Join(factory.config.ParentDirectory, directoryName)

			return &Store{storeState: &storeState{
				config: factory.config,
				removeAll: func(string) error {
					return factory.removeAll(root, directoryName)
				},
				root:          root,
				directory:     directory,
				directoryName: directoryName,
				closing:       true,
				cleanupOnly:   true,
			}}, err
		}
		_ = root.Close()
		clear(identity)

		return nil, err
	}
	directory := filepath.Join(factory.config.ParentDirectory, directoryName)
	state := &storeState{
		config:        factory.config,
		nameEntropy:   factory.nameEntropy,
		root:          root,
		directory:     directory,
		directoryName: directoryName,
		cipher:        authenticatedCipher,
		identity:      identity,
		nonceDomain:   nonceDomain,
		buffer: recordBuffer{
			data: make(
				[]byte,
				0,
				factory.config.RecordBytes*factory.config.ChunkRecords,
			),
			recordBytes: factory.config.RecordBytes,
		},
	}
	state.createTemp = func(string, string) (chunkFile, error) {
		return createRootTemporaryChunk(root, directoryName, state.nameEntropy)
	}
	state.openFile = func(path string) (chunkFile, error) {
		return root.Open(filepath.Join(directoryName, filepath.Base(path)))
	}
	state.remove = func(path string) error {
		return root.Remove(filepath.Join(directoryName, filepath.Base(path)))
	}
	state.removeAll = func(string) error {
		return root.RemoveAll(directoryName)
	}

	return &Store{storeState: state}, nil
}

// Store owns encrypted temporary chunks for one sort; obtain it from Factory.Open.
// Its zero value is invalid. Copies of Store share the same lifecycle and
// resources; they are not independent sorts. The caller owns Close.
//
// One lifecycle operation is admitted at a time. While open, overlap/reentry
// returns ErrConcurrentUse; input/iteration after closing begins returns
// ErrClosed. Close also rejects overlap and does not interrupt active
// work. No store lock is held across IO or callbacks and no goroutine starts.
// Callback records are borrowed plaintext, cleared when the callback returns.
type Store struct {
	*storeState
}

type storeState struct {
	// mutex owns lifecycle admission; busy grants exclusive access to mutable
	// records/IO state without holding the mutex across storage or user code.
	mutex         sync.Mutex
	config        Config
	nameEntropy   io.Reader
	createTemp    func(string, string) (chunkFile, error)
	openFile      func(string) (chunkFile, error)
	remove        func(string) error
	removeAll     func(string) error
	root          rootDirectory
	directory     string
	directoryName string
	cipher        cipher.AEAD
	identity      []byte
	nonceDomain   uint64
	nonceCount    uint64
	buffer        recordBuffer
	chunks        []string
	total         int
	finalized     bool
	busy          bool
	closing       bool
	closed        bool
	cleanupOnly   bool
}

// String prevents records, cipher state, and temporary paths from entering
// text logs.
func (Store) String() string {
	return redacted
}

// GoString prevents records, cipher state, and temporary paths from entering
// debug representations.
func (Store) GoString() string {
	return redacted
}

// LogValue prevents records, cipher state, and temporary paths from entering
// slog.
func (Store) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON prevents records, cipher state, and temporary paths from entering
// JSON.
func (Store) MarshalJSON() ([]byte, error) {
	return []byte(redactedJSON), nil
}

// Add copies one exact Config.RecordBytes record into the bounded chunk. The
// caller may reuse the slice after return, but must not mutate it during Add.
// A full chunk is sorted and encrypted to disk before Add returns. Nil store
// or context returns ErrInvalidConfiguration; wrong width returns
// ErrInvalidRecord, and exceeding MaximumRecords returns ErrRecordLimit.
// Lifecycle errors are ErrConcurrentUse, ErrClosed and ErrFinalized.
//
// A failed Add does not accept the new record and preserves previously accepted
// records. It may be retried after cancellation, entropy or storage failure
// unless partial-file cleanup fails, which permits only Close. Spill retries
// consume new nonce counters rather than reusing a failed attempt's nonce.
// Context errors pass through; owned IO failures use payload-free sentinels.
func (store *Store) Add(ctx context.Context, record []byte) error {
	if store == nil || ctx == nil {
		return ErrInvalidConfiguration
	}
	if err := store.begin(true); err != nil {
		return err
	}
	defer store.finish()
	if len(record) != store.config.RecordBytes {
		return ErrInvalidRecord
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if store.total >= store.config.MaximumRecords {
		return ErrRecordLimit
	}

	store.buffer.append(record)
	store.total++
	if store.buffer.Len() < store.config.ChunkRecords {
		return nil
	}
	if err := store.spill(ctx); err != nil {
		store.buffer.removeOne(record)
		store.total--

		return err
	}

	return nil
}

// ForEachSorted finalizes input and calls yield synchronously for every record
// in ascending full-record byte order, preserving duplicates. There is no custom
// comparator or key-only stable-sort contract. Empty input succeeds without a
// callback. A nil store, context or yield returns ErrInvalidConfiguration.
//
// Each callback slice is cleared on return, including panic; copy before
// retaining it. Callback errors and context errors pass through unchanged and
// callback panics propagate after plaintext is cleared and reader closure is
// attempted. A blocked callback can delay cancellation; reentrant lifecycle
// calls on this store are rejected, not awaited. Cancellation after the final
// checkpoint need not turn otherwise successful iteration into an error.
//
// Output is not atomic: corruption, storage, cancellation or callback failure
// can follow records already yielded. Reader-close failure can return ErrStorage
// even after complete output; it does not override an earlier iteration error.
// Once pending input is spilled, the store remains finalized even on failure,
// including empty input. Further Add/ForEachSorted calls return ErrFinalized.
// A pre-finalization failure is retryable only if cleanup has not sealed the
// store. The caller must Close on every path; iteration does not delete chunks.
func (store *Store) ForEachSorted(
	ctx context.Context,
	yield func([]byte) error,
) (result error) {
	if store == nil || ctx == nil || yield == nil {
		return ErrInvalidConfiguration
	}
	if err := store.begin(true); err != nil {
		return err
	}
	defer store.finish()
	if err := ctx.Err(); err != nil {
		return err
	}
	if store.buffer.Len() > 0 {
		if err := store.spill(ctx); err != nil {
			return err
		}
	}
	store.finalized = true
	if len(store.chunks) == 0 {
		return nil
	}

	readers, err := store.openReaders()
	if err != nil {
		return err
	}
	defer func() {
		closeErr := closeReaders(readers)
		if result == nil {
			result = closeErr
		}
	}()

	return merge(ctx, readers, yield)
}

// Close clears buffered plaintext, removes the work directory and releases the
// rooted handle. A nil Store succeeds; an unconstructed zero Store returns
// ErrInvalidConfiguration. Close rejects active operations with ErrConcurrentUse
// instead of waiting or interrupting them. It has no context and can block in IO.
//
// Once closing begins, Add/ForEachSorted return ErrClosed. A removal failure
// returns ErrStorage and leaves only Close retryable. After successful removal,
// state is finalized even if rooted-handle closure reports ErrStorage; later
// Close calls return nil without retrying that handle. Otherwise repeated Close
// is idempotent. Dropping cipher references is not secure key-memory erasure,
// and process death can bypass cleanup entirely.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	if store.storeState == nil {
		return ErrInvalidConfiguration
	}
	store.mutex.Lock()
	if store.closed {
		store.mutex.Unlock()

		return nil
	}
	if store.busy {
		store.mutex.Unlock()

		return ErrConcurrentUse
	}
	if !store.validUnlocked() {
		store.mutex.Unlock()

		return ErrInvalidConfiguration
	}
	store.busy = true
	store.closing = true
	store.mutex.Unlock()
	store.buffer.clear()
	if err := store.removeAll(store.directory); err != nil {
		store.finish()

		return ErrStorage
	}
	rootCloseErr := store.root.Close()
	store.directory = ""
	store.directoryName = ""
	store.root = nil
	store.chunks = nil
	store.cipher = nil
	store.nameEntropy = nil
	clear(store.identity)
	store.identity = nil
	store.mutex.Lock()
	store.closing = false
	store.closed = true
	store.busy = false
	store.mutex.Unlock()
	if rootCloseErr != nil {
		return ErrStorage
	}

	return nil
}

func (store *Store) valid() bool {
	if store == nil || store.storeState == nil {
		return false
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.validUnlocked()
}

func (store *Store) validUnlocked() bool {
	if store.cleanupOnly {
		return validConfig(store.config) && store.removeAll != nil &&
			store.root != nil && store.directory != "" && store.directoryName != ""
	}

	return validConfig(store.config) &&
		store.nameEntropy != nil &&
		store.createTemp != nil &&
		store.openFile != nil &&
		store.remove != nil &&
		store.removeAll != nil &&
		store.root != nil &&
		store.directory != "" &&
		store.directoryName != "" &&
		store.cipher != nil &&
		len(store.identity) == storeIdentityBytes &&
		store.buffer.recordBytes == store.config.RecordBytes
}

func (store *Store) begin(requireUnfinalized bool) error {
	if store.storeState == nil {
		return ErrInvalidConfiguration
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if store.closed || store.closing {
		return ErrClosed
	}
	if store.busy {
		return ErrConcurrentUse
	}
	if requireUnfinalized && store.finalized {
		return ErrFinalized
	}
	if !store.validUnlocked() {
		return ErrInvalidConfiguration
	}
	store.busy = true

	return nil
}

func (store *Store) finish() {
	store.mutex.Lock()
	store.busy = false
	store.mutex.Unlock()
}

func (store *Store) seal() {
	store.mutex.Lock()
	store.closing = true
	store.mutex.Unlock()
}

// Check the byte product by division before allocation/multiplication; rounded
// chunk admission bounds merge fan-in even for a partly filled final chunk.
func validConfig(config Config) bool {
	if config.ParentDirectory == "" ||
		!filepath.IsAbs(config.ParentDirectory) ||
		config.RecordBytes <= 0 ||
		config.RecordBytes > MaximumRecordBytes ||
		config.ChunkRecords <= 0 ||
		config.ChunkRecords > MaximumChunkRecords ||
		config.MaximumRecords < config.ChunkRecords ||
		config.RecordBytes > MaximumChunkBytes/config.ChunkRecords {
		return false
	}
	chunks := config.MaximumRecords / config.ChunkRecords
	if config.MaximumRecords%config.ChunkRecords != 0 {
		chunks++
	}

	return chunks <= MaximumMergeFiles
}

func validatedParent(parent string) (os.FileInfo, error) {
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() ||
		info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0o077 != 0 {
		return nil, ErrUnsafeParent
	}

	return info, nil
}

func sameSafeDirectory(expected os.FileInfo, actual os.FileInfo) bool {
	return expected != nil && actual != nil && actual.IsDir() &&
		actual.Mode().Perm()&0o077 == 0 && os.SameFile(expected, actual)
}

type nonceDomainAllocator struct {
	mutex       sync.Mutex
	entropy     io.Reader
	initialized bool
	seed        uint64
	next        uint64
}

func allocateNonceDomain() (uint64, bool) {
	return processNonceDomains.allocate()
}

func (allocator *nonceDomainAllocator) allocate() (uint64, bool) {
	allocator.mutex.Lock()
	defer allocator.mutex.Unlock()
	if !allocator.initialized {
		var seed [8]byte
		if _, err := io.ReadFull(allocator.entropy, seed[:]); err != nil {
			clear(seed[:])

			return 0, false
		}
		allocator.seed = binary.BigEndian.Uint64(seed[:])
		allocator.initialized = true
		clear(seed[:])
	}
	if allocator.next == ^uint64(0) {
		return 0, false
	}
	allocator.next++

	// XOR with one process seed is bijective over the sequence. It separates
	// stores in this process even with repeated store entropy, not across
	// restarts/processes; callers still need independent dataset keys.
	return allocator.seed ^ allocator.next, true
}

func (store *Store) spill(ctx context.Context) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	sort.Sort(&store.buffer)
	file, err := store.createTemp(store.directory, chunkPrefix)
	if err != nil {
		if errors.Is(err, ErrEntropy) {
			return ErrEntropy
		}

		return ErrStorage
	}
	path := file.Name()
	complete := false
	defer func() {
		if !complete {
			closeErr := file.Close()
			removeErr := store.remove(path)
			if closeErr != nil || removeErr != nil {
				store.seal()
				result = ErrStorage
			}
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return ErrStorage
	}

	chunkIndex := uint64(len(store.chunks))
	for recordIndex := 0; recordIndex < store.buffer.Len(); recordIndex++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		nonce := make([]byte, store.cipher.NonceSize())
		if store.nonceCount > uint64(^uint32(0)) {
			clear(nonce)

			return ErrEntropy
		}
		binary.BigEndian.PutUint64(nonce[:nonceCounterOffset], store.nonceDomain)
		binary.BigEndian.PutUint32(
			nonce[nonceCounterOffset:],
			uint32(store.nonceCount),
		)
		// Consume before any write: a failed partial spill must never reuse this
		// nonce when accepted records are retried (fault_campaign_test.go).
		store.nonceCount++
		plaintext := store.buffer.record(recordIndex)
		aad := additionalData(
			store.identity,
			store.nonceDomain,
			chunkIndex,
			uint64(recordIndex),
			uint64(store.config.RecordBytes), // #nosec G115 -- Factory and Store admission require 1 <= RecordBytes <= MaximumRecordBytes (1 MiB).
		)
		encrypted := store.cipher.Seal(nil, nonce, plaintext, aad)
		record := append(nonce, encrypted...)
		if err := writeFull(file, record); err != nil {
			clear(nonce)
			clear(encrypted)
			clear(record)

			return ErrStorage
		}
		clear(nonce)
		clear(encrypted)
		clear(record)
	}
	if err := file.Sync(); err != nil {
		return ErrStorage
	}
	if err := file.Close(); err != nil {
		return ErrStorage
	}
	// Only a synced, closed file enters the merge list. Uncommitted files are
	// removed by the defer; failed cleanup seals admission to bound residue.
	complete = true
	store.chunks = append(store.chunks, path)
	store.buffer.clear()

	return nil
}

func (store *Store) openReaders() ([]*chunkReader, error) {
	readers := make([]*chunkReader, 0, len(store.chunks))
	for index, path := range store.chunks {
		file, err := store.openFile(filepath.Clean(path))
		if err != nil {
			_ = closeReaders(readers)

			return nil, ErrStorage
		}
		readers = append(readers, &chunkReader{
			file:            file,
			cipher:          store.cipher,
			identity:        store.identity,
			nonceDomain:     store.nonceDomain,
			recordBytes:     store.config.RecordBytes,
			chunkIndex:      uint64(index),
			expectedRecords: uint64(store.expectedChunkRecords(index)), // #nosec G115 -- Finalized successful chunks contain 1..ChunkRecords records; admission caps ChunkRecords at one million and fan-in at 64.
		})
	}

	return readers, nil
}

func (store *Store) expectedChunkRecords(index int) int {
	return min(
		store.config.ChunkRecords,
		store.total-index*store.config.ChunkRecords,
	)
}

// One plaintext record per reader is held in the heap. Pop transfers clearing
// responsibility to the callback scope; queue.clear owns remaining plaintext
// on every error or panic. Output already yielded cannot be rolled back.
func merge(
	ctx context.Context,
	readers []*chunkReader,
	yield func([]byte) error,
) error {
	queue := make(recordHeap, 0, len(readers))
	defer queue.clear()
	for index, reader := range readers {
		record, err := reader.next()
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return err
		}
		heap.Push(&queue, heapRecord{record: record, reader: index})
	}

	for queue.Len() > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := heap.Pop(&queue).(heapRecord)
		yieldErr := func() error {
			defer clear(current.record)

			return yield(current.record)
		}()
		if yieldErr != nil {
			return yieldErr
		}
		next, err := readers[current.reader].next()
		switch {
		case errors.Is(err, io.EOF):
		case err != nil:
			return err
		default:
			heap.Push(
				&queue,
				heapRecord{record: next, reader: current.reader},
			)
		}
	}

	return nil
}

// chunkReader trusts only admitted in-memory sizes/counts, never storage length
// headers. Its AEAD metadata pins each record to a store, chunk and ordinal.
type chunkReader struct {
	file            chunkFile
	cipher          cipher.AEAD
	identity        []byte
	nonceDomain     uint64
	recordBytes     int
	chunkIndex      uint64
	recordIndex     uint64
	expectedRecords uint64
}

// chunkFile is the owned synchronous IO seam. Spill closes/removes uncommitted
// files; iteration attempts to close every reader, including on callback panic.
type chunkFile interface {
	io.Reader
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

// rootDirectory anchors relative names to one revalidated directory identity.
// Production adapters must preserve containment through ancestor renames; this
// seam is not permission to replace rooted operations with unrestricted paths.
type rootDirectory interface {
	Stat(string) (os.FileInfo, error)
	Mkdir(string, os.FileMode) error
	Chmod(string, os.FileMode) error
	Open(string) (*os.File, error)
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Remove(string) error
	RemoveAll(string) error
	Close() error
}

func createStoreDirectory(
	ctx context.Context,
	root rootDirectory,
	entropy io.Reader,
	mkdir func(rootDirectory, string, os.FileMode) error,
	chmod func(rootDirectory, string, os.FileMode) error,
	removeAll func(rootDirectory, string) error,
) (string, bool, error) {
	for range temporaryAttempts {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		name, err := temporaryName(entropy, storePrefix)
		if err != nil {
			return "", false, err
		}
		if err := mkdir(root, name, 0o700); err == nil {
			if err := chmod(root, name, 0o700); err != nil {
				if removeErr := removeAll(root, name); removeErr != nil {
					return name, true, ErrStorage
				}

				return "", false, ErrStorage
			}
			info, err := root.Stat(name)
			if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
				if removeErr := removeAll(root, name); removeErr != nil {
					return name, true, ErrStorage
				}

				return "", false, ErrStorage
			}

			return name, false, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", false, ErrStorage
		}
	}

	return "", false, ErrStorage
}

func createRootTemporaryChunk(
	root rootDirectory,
	directory string,
	entropy io.Reader,
) (chunkFile, error) {
	for range temporaryAttempts {
		name, err := temporaryName(entropy, chunkPrefix)
		if err != nil {
			return nil, err
		}
		file, err := root.OpenFile(
			filepath.Join(directory, name),
			os.O_RDWR|os.O_CREATE|os.O_EXCL,
			0o600,
		)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, ErrStorage
		}
	}

	return nil, ErrStorage
}

func temporaryName(entropy io.Reader, prefix string) (string, error) {
	random := make([]byte, temporaryNameBytes)
	if _, err := io.ReadFull(entropy, random); err != nil {
		clear(random)

		return "", ErrEntropy
	}
	name := prefix + hex.EncodeToString(random)
	clear(random)

	return name, nil
}

func (reader *chunkReader) next() ([]byte, error) {
	if reader.recordIndex == reader.expectedRecords {
		// EOF must occur at the expected record count: authenticating individual
		// records alone would miss removal of a whole final record or appended data.
		var trailing [1]byte
		count, err := reader.file.Read(trailing[:])
		if count == 0 && errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if count == 0 && err != nil {
			return nil, ErrStorage
		}

		return nil, ErrCorrupt
	}
	encryptedRecordBytes := reader.cipher.NonceSize() +
		reader.recordBytes +
		reader.cipher.Overhead()
	record := make([]byte, encryptedRecordBytes)
	_, err := io.ReadFull(reader.file, record)
	if err != nil {
		clear(record)
		if !errors.Is(err, io.EOF) &&
			!errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrStorage
		}

		return nil, ErrCorrupt
	}
	nonce := record[:reader.cipher.NonceSize()]
	aad := additionalData(
		reader.identity,
		reader.nonceDomain,
		reader.chunkIndex,
		reader.recordIndex,
		uint64(reader.recordBytes), // #nosec G115 -- Private readers receive the admitted positive RecordBytes <= 1 MiB; stored bytes cannot change this field.
	)
	plaintext, err := reader.cipher.Open(
		nil,
		nonce,
		record[reader.cipher.NonceSize():],
		aad,
	)
	clear(record)
	if err != nil || len(plaintext) != reader.recordBytes {
		clear(plaintext)

		return nil, ErrCorrupt
	}
	reader.recordIndex++

	return plaintext, nil
}

func closeReaders(readers []*chunkReader) error {
	var closeFailed bool
	for _, reader := range readers {
		if err := reader.file.Close(); err != nil {
			closeFailed = true
		}
	}
	if closeFailed {
		return ErrStorage
	}

	return nil
}

// The private extsort1 AAD binds identity, nonce domain, chunk, ordinal and
// record width in big-endian order. Source contract tests reject metadata and
// cross-store/chunk substitutions; see docs/architecture.md for the format.
func additionalData(
	identity []byte,
	nonceDomain uint64,
	chunkIndex uint64,
	recordIndex uint64,
	recordBytes uint64,
) []byte {
	result := make([]byte, len(aadVersion)+storeIdentityBytes+32)
	copy(result, aadVersion)
	copy(result[len(aadVersion):], identity)
	offset := len(aadVersion) + storeIdentityBytes
	binary.BigEndian.PutUint64(result[offset:offset+8], nonceDomain)
	binary.BigEndian.PutUint64(result[offset+8:offset+16], chunkIndex)
	binary.BigEndian.PutUint64(result[offset+16:offset+24], recordIndex)
	binary.BigEndian.PutUint64(result[offset+24:], recordBytes)

	return result
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}

	return nil
}

type recordBuffer struct {
	data        []byte
	recordBytes int
}

func (buffer *recordBuffer) Len() int {
	return len(buffer.data) / buffer.recordBytes
}

func (buffer *recordBuffer) Less(left int, right int) bool {
	return bytes.Compare(buffer.record(left), buffer.record(right)) < 0
}

func (buffer *recordBuffer) Swap(left int, right int) {
	leftRecord := buffer.record(left)
	rightRecord := buffer.record(right)
	for index := range leftRecord {
		leftRecord[index], rightRecord[index] =
			rightRecord[index], leftRecord[index]
	}
}

func (buffer *recordBuffer) append(record []byte) {
	buffer.data = append(buffer.data, record...)
}

// spill sorts before it can fail, so rollback must locate a matching value
// rather than truncate the last slot. Equal records are byte-identical; removing
// any one copy restores multiplicity without inventing input identity.
func (buffer *recordBuffer) removeOne(record []byte) {
	for index := 0; index < buffer.Len(); index++ {
		if !bytes.Equal(buffer.record(index), record) {
			continue
		}
		start := index * buffer.recordBytes
		next := start + buffer.recordBytes
		copy(buffer.data[start:], buffer.data[next:])
		newLength := len(buffer.data) - buffer.recordBytes
		clear(buffer.data[newLength:])
		buffer.data = buffer.data[:newLength]

		return
	}
}

func (buffer *recordBuffer) record(index int) []byte {
	start := index * buffer.recordBytes

	return buffer.data[start : start+buffer.recordBytes]
}

func (buffer *recordBuffer) clear() {
	clear(buffer.data)
	buffer.data = buffer.data[:0]
}

type heapRecord struct {
	record []byte
	reader int
}

type recordHeap []heapRecord

func (records recordHeap) Len() int {
	return len(records)
}

func (records recordHeap) Less(left int, right int) bool {
	comparison := bytes.Compare(
		records[left].record,
		records[right].record,
	)
	if comparison == 0 {
		// Reader index gives a strict deterministic tie-break. Sorting compares
		// full records, so equal values have no distinct consumer-visible key.
		return records[left].reader < records[right].reader
	}

	return comparison == -1
}

func (records recordHeap) Swap(left int, right int) {
	records[left], records[right] = records[right], records[left]
}

func (records *recordHeap) Push(value any) {
	*records = append(*records, value.(heapRecord))
}

func (records *recordHeap) Pop() any {
	old := *records
	last := len(old) - 1
	value := old[last]
	old[last] = heapRecord{}
	*records = old[:last]

	return value
}

func (records *recordHeap) clear() {
	for index := range *records {
		clear((*records)[index].record)
		(*records)[index] = heapRecord{}
	}
	*records = nil
}
