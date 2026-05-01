package ocgcore

import (
	"fmt"
	"github.com/ebitengine/purego"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

type OCGApi struct {
	handle            uintptr
	rootPath          string
	scriptDirectory   string
	databaseFile      string
	buffer            []byte
	CreateDuel        func(seed int32) uintptr
	StartDuel         func(pduel uintptr, options int32)
	EndDuel           func(pduel uintptr)
	SetPlayerInfo     func(pduel uintptr, playerId, LP, startCount, drawCount int32)
	GetLogMessage     func(pduel uintptr, buf []byte)
	GetMessage        func(pduel uintptr, buf []byte) int32
	Process           func(pduel uintptr) uint32
	NewCard           func(pduel uintptr, code uint32, owner, playerid, location, sequence, position uint8)
	NewTagCard        func(pduel uintptr, code uint32, owner, location uint8)
	QueryCard         func(pduel uintptr, playerid, location, sequence uint8, queryFlag int32, buf []byte, useCache int32) int32
	QueryFieldCount   func(pduel uintptr, playerid, location uint8) int32
	QueryFieldCard    func(pduel uintptr, playerId, location uint8, queryFlag uint32, buf []byte, useCache int32) int32
	QueryFieldInfo    func(pduel uintptr, buf []byte) int32
	SetResponseI      func(pduel uintptr, value int32)
	SetResponseB      func(pduel uintptr, buf []byte)
	PreloadScript     func(pduel uintptr, script string, len int32) int32
	SetScriptReader   func(f uintptr)
	SetCardReader     func(f uintptr)
	SetMessageHandler func(f uintptr)

	scriptReader   ScriptReader
	cardReader     CardReader
	messageHandler MessageHandler
}

// YGOCardData kept for backward compatibility
type YGOCardData struct {
	Id         uint32
	Alias      uint32
	Setcode    int64
	Type       uint32
	Level      uint32
	Attribute  uint32
	Race       uint32
	Attack     int64
	Defense    int64
	LScale     uint32
	RScale     uint32
	LinkMarker uint32
}

// Public callback types – user-friendly, no CGO
type (
	ScriptReader   func(scriptName string) []byte
	CardReader     func(cardId uint32) *CardData
	MessageHandler func(pduel uintptr, msgSize uint32)
)

type Option func(api *OCGApi)

func WithRootPath(path string) Option {
	return func(api *OCGApi) { api.rootPath = path }
}
func WithScriptDirectory(path string) Option {
	return func(api *OCGApi) { api.scriptDirectory = path }
}
func WithDatabaseFile(path string) Option {
	return func(api *OCGApi) { api.databaseFile = path }
}
func WithScriptReader(fn ScriptReader) Option {
	return func(api *OCGApi) { api.scriptReader = fn }
}
func WithCardReader(fn CardReader) Option {
	return func(api *OCGApi) { api.cardReader = fn }
}
func WithMessageHandler(fn MessageHandler) Option {
	return func(api *OCGApi) { api.messageHandler = fn }
}

var API *OCGApi

var libCandidates = []string{
	"ocgcore.dll",
	"libocgcore.so",
	"libocgcore.dylib",
}

func init() {
	if runtime.GOOS == "windows" {
		libCandidates = []string{"ocgcore.dll"}
	} else if runtime.GOOS == "darwin" {
		libCandidates = []string{"libocgcore.dylib", "libocgcore.so"}
	} else {
		libCandidates = []string{"libocgcore.so", "ocgcore.dll"}
	}
}

func registerFunctions(api *OCGApi, libc uintptr) {
	purego.RegisterLibFunc(&api.CreateDuel, libc, "create_duel")
	purego.RegisterLibFunc(&api.StartDuel, libc, "start_duel")
	purego.RegisterLibFunc(&api.EndDuel, libc, "end_duel")
	purego.RegisterLibFunc(&api.SetPlayerInfo, libc, "set_player_info")
	purego.RegisterLibFunc(&api.GetLogMessage, libc, "get_log_message")
	purego.RegisterLibFunc(&api.GetMessage, libc, "get_message")
	purego.RegisterLibFunc(&api.Process, libc, "process")
	purego.RegisterLibFunc(&api.NewCard, libc, "new_card")
	purego.RegisterLibFunc(&api.NewTagCard, libc, "new_tag_card")
	purego.RegisterLibFunc(&api.QueryCard, libc, "query_card")
	purego.RegisterLibFunc(&api.QueryFieldCount, libc, "query_field_count")
	purego.RegisterLibFunc(&api.QueryFieldCard, libc, "query_field_card")
	purego.RegisterLibFunc(&api.QueryFieldInfo, libc, "query_field_info")
	purego.RegisterLibFunc(&api.SetResponseI, libc, "set_responsei")
	purego.RegisterLibFunc(&api.SetResponseB, libc, "set_responseb")
	purego.RegisterLibFunc(&api.PreloadScript, libc, "preload_script")
	purego.RegisterLibFunc(&api.SetScriptReader, libc, "set_script_reader")
	purego.RegisterLibFunc(&api.SetCardReader, libc, "set_card_reader")
	purego.RegisterLibFunc(&api.SetMessageHandler, libc, "set_message_handler")
}

// Init initializes the OCG wrapper using purego (no CGO).
func Init(opts ...Option) error {
	ocgApi := &OCGApi{
		rootPath:        ".",
		scriptDirectory: "script",
		databaseFile:    "cards.cdb",
		buffer:          make([]byte, 128*1024),
	}
	ocgApi.scriptReader = ocgApi.defaultScriptReader
	ocgApi.cardReader = ocgApi.defaultCardReader
	ocgApi.messageHandler = ocgApi.defaultOnMessageHandler
	API = ocgApi
	for _, opt := range opts {
		opt(API)
	}

	var err error
	for _, name := range libCandidates {
		libPath := filepath.Join(ocgApi.rootPath, name)
		ocgApi.handle, err = openLibrary(libPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("failed to load ocgcore library: %w", err)
	}

	registerFunctions(ocgApi, ocgApi.handle)
	ocgApi.SetCallback()
	return nil
}

func (o *OCGApi) SetCallback() {
	o.SetScriptReader(purego.NewCallback(scriptReaderCallback))
	o.SetCardReader(purego.NewCallback(cardReaderCallback))
	o.SetMessageHandler(purego.NewCallback(messageHandlerCallback))
}

func (o *OCGApi) Dispose() {
	if o.handle != 0 {
		_ = closeLibrary(o.handle)
		o.handle = 0
	}
}

// ------------------------------------------------------------------
// Internal C-compatible callbacks (invoked by the ocgcore .so/.dll)
// ------------------------------------------------------------------

// C signature: unsigned char* script_reader(const char* name, int* len)
func scriptReaderCallback(scriptName *byte, length *int32) uintptr {
	name := cStringToGoString(scriptName)
	data := API.scriptReader(name)
	if len(data) == 0 {
		*length = 0
		return 0
	}
	*length = int32(len(data))
	return uintptr(unsafe.Pointer(&data[0]))
}

// C signature: uint32_t card_reader(uint32_t code, card_data* data)
func cardReaderCallback(code uint32, data *CardData) uintptr {
	card := API.cardReader(code)
	if card == nil {
		return 0
	}
	*data = *card
	return uintptr(card.Code)
}

// C signature: uint32_t message_handler(intptr_t pduel, uint32_t size)
func messageHandlerCallback(pduel uintptr, size uint32) uintptr {
	API.messageHandler(pduel, size)
	return 0
}

// ------------------------------------------------------------------
// Default implementations
// ------------------------------------------------------------------

func (o *OCGApi) defaultScriptReader(scriptName string) []byte {
	fmt.Println("Loading script:", scriptName)
	scriptPath := filepath.Join(API.scriptDirectory, scriptName)
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Println("Error reading script file:", err)
		return nil
	}
	return data
}

func (o *OCGApi) defaultCardReader(code uint32) *CardData {
	return nil
}

func (o *OCGApi) defaultOnMessageHandler(duelPtr uintptr, msgSize uint32) {
	duelLock.Lock()
	defer duelLock.Unlock()
	duel, has := duels[duelPtr]
	if has {
		duel.OnMessage(msgSize)
	}
}

// GoString converts a C string pointer to a Go string.
func GoString(c uintptr) string {
	ptr := *(*unsafe.Pointer)(unsafe.Pointer(&c))
	if ptr == nil {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Add(ptr, uintptr(length))) == '\x00' {
			break
		}
		length++
	}
	return string(unsafe.Slice((*byte)(ptr), length))
}

func cStringToGoString(p *byte) string {
	if p == nil {
		return ""
	}
	var s []byte
	for {
		if *p == 0 {
			break
		}
		s = append(s, *p)
		p = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 1))
	}
	return string(s)
}
