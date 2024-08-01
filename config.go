package env86

type Config struct {
	V86Config
	NoConsole     bool
	EnableTTY     bool
	ColdBoot      bool
	SaveOnExit    bool
	ExitOnPattern string
	EnableNetwork bool
	ChromeDP      bool
	ConsoleAddr   string
}

type V86Config struct {
	WasmPath                    string           `json:"wasm_path,omitempty"`
	BIOS                        *ImageConfig     `json:"bios,omitempty"`
	VGABIOS                     *ImageConfig     `json:"vga_bios,omitempty"`
	MemorySize                  int              `json:"memory_size,omitempty"`
	VGAMemorySize               int              `json:"vga_memory_size,omitempty"`
	InitialState                *ImageConfig     `json:"initial_state,omitempty"`
	NetworkRelayURL             string           `json:"network_relay_url,omitempty"`
	Filesystem                  FilesystemConfig `json:"filesystem,omitempty"`
	Autostart                   bool             `json:"autostart,omitempty"`
	BZImageInitrdFromFilesystem bool             `json:"bzimage_initrd_from_filesystem,omitempty"`
	ScreenContainer             string           `json:"screen_container,omitempty"`
	Cmdline                     string           `json:"cmdline,omitempty"`
	DisableKeyboard             bool             `json:"disable_keyboard,omitempty"`
	DisableMouse                bool             `json:"disable_mouse,omitempty"`
	HDA                         *ImageConfig     `json:"hda,omitempty"`
	FDA                         *ImageConfig     `json:"fda,omitempty"`
	CDROM                       *ImageConfig     `json:"cdrom,omitempty"`
	BZImage                     *ImageConfig     `json:"bzimage,omitempty"`
	Initrd                      *ImageConfig     `json:"initrd,omitempty"`
	SerialContainer             string           `json:"serial_container,omitempty"`
	PreserveMAC                 bool             `json:"preserve_mac_from_state_image,omitempty"`
}

type FilesystemConfig struct {
	BaseURL string `json:"baseurl,omitempty"`
	BaseFS  string `json:"basefs,omitempty"`
}

type ImageConfig struct {
	URL   string `json:"url,omitempty"`
	Async bool   `json:"async,omitempty"`
	Size  int    `json:"size,omitempty"`
}
