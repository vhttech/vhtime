# Kiến trúc Module

## Luồng dữ liệu tổng quan

```
IBus Daemon
    │  D-Bus
    ▼
main.go ──► GetIBusEngineCreator()
                │
                ├── config.LoadConfig()          → config/config.go
                ├── bamboo.NewEngine()           → vendor/bamboo-core
                └── NewIbusBambooEngine()        → engine.go
                        │
                        ├── ProcessKeyEvent()    ← IBus key event
                        │       │
                        │       ├── preeditProcessKeyEvent()    engine_preedit.go
                        │       ├── bsProcessKeyEvent()         engine_backspace.go
                        │       ├── emojiProcessKeyEvent()      engine_emoji.go
                        │       └── hexProcessKeyEvent()        engine_hexadecimal.go
                        │
                        ├── commitText() / updatePreedit()
                        │       └── → IBus → Application
                        │
                        └── prop.go  (menu UI trên thanh taskbar)
```

## Các module chính

### 1. Engine Core (`engine.go`)
- `IBusBambooEngine` là struct trung tâm, embed `IEngine` (IBus base engine) và `bamboo.IEngine` (preeditor)
- `ProcessKeyEvent()` là handler chính — kiểm tra state rồi dispatch sang mode tương ứng
- Có mutex (`sync.Mutex`) để thread-safe, vì mouse capture và key event chạy song song

### 2. bamboo-core (vendor)
- Thư viện xử lý âm vị học tiếng Việt
- `bamboo.IEngine` interface: `ProcessKey()`, `RemoveLastChar()`, `GetProcessedString()`, `CanProcessKey()`
- Hỗ trợ nhiều input method (Telex/VNI/VIQR/…) qua `InputMethodDefinition`
- `ParseInputMethod()` nạp định nghĩa từ `bamboo.xml` / built-in

### 3. Input Mode Dispatcher
Engine chọn processor dựa vào `config.DefaultInputMode`:

| Mode | Hàm xử lý | File |
|------|-----------|------|
| PreeditIM | `preeditProcessKeyEvent` | engine_preedit.go |
| SurroundingTextIM | `bsProcessKeyEvent` | engine_backspace.go |
| BackspaceForwardingIM | `bsProcessKeyEvent` | engine_backspace.go |
| ShiftLeftForwardingIM | `bsProcessKeyEvent` | engine_backspace.go |
| ForwardAsCommitIM | `bsProcessKeyEvent` | engine_backspace.go |
| XTestFakeKeyEventIM | `bsProcessKeyEvent` + XTest | engine_backspace.go + x11.go |
| UsIM | bypass (English/exclusion) | engine_utils.go |

### 4. Window Class Detection
Engine theo dõi cửa sổ đang focus để áp dụng input mode mapping:
- **X11**: `x11_introspector.c` → `x11GetFocusWindowClass()` (XGetInputFocus + XFetchName)
- **Wayland (non-GNOME)**: `wl_introspector.go` → `wlGetFocusWindowClass()` (wlr-foreign-toplevel protocol)
- **GNOME**: `gnome_introspector.go` → D-Bus `org.gnome.Shell` để lấy focused app

### 5. X11 Native Layer (`x11.go` + C files)
CGo bridge cho các thao tác cần X11 native:
- **Clipboard**: `x11Copy()`, `x11Paste()` — x11_clipboard.c
- **Fake key**: `x11SendBackspace()`, `x11SendShiftLeft()` — x11_keyboard.c (XTest extension)
- **Mouse capture**: `mouse_capture_init/exit/unlock()` — x11_mouse.c
- **Key recording**: X11 Record extension — x11_record.c (detect keypress globally)

### 6. MacroTable (`mactab.go`)
- Load file text `~/.config/ibus-bamboo/ibus-bamboo.macro.text`
- Format: `keyword:replacement` mỗi dòng
- Hỗ trợ auto-capitalize khi macro được nhận ra

### 7. Emoji Engine (`emoji.go` + `engine_emoji.go`)
- Load `data/emojione.json` vào Trie (`trie.go`) để tìm kiếm nhanh
- Khi user gõ `:keyword`, hiện lookup table để chọn emoji

### 8. UI (`ui/ui.go` + GTK3 C)
- `ui.OpenGUI()` mở cửa sổ cấu hình GTK3
- Export functions từ Go sang C: `saveFlags`, `saveConfigText`, `saveMacroText`, `saveInputMode`
- Trigger bằng flag `-gui` hoặc qua IBus property menu

## Dependency Graph

```
main
 ├── config          (không có dep ngoài bamboo-core)
 ├── ui              (dep GTK3 qua CGo)
 ├── goibus          (IBus D-Bus binding)
 ├── bamboo-core     (Vietnamese NLP)
 ├── godbus/dbus/v5  (D-Bus client)
 ├── dkolbly/wl      (Wayland protocol client)
 └── golang.org/x/net/context
```
