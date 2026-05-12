# Config & Flags

## Config struct (config/config.go)

```go
type Config struct {
    InputMethod            string                              // "Telex", "VNI", "VIQR", ...
    InputMethodDefinitions map[string]bamboo.InputMethodDefinition
    OutputCharset          string                              // "Unicode", "TCVN3", "VNI", ...
    Flags                  uint                                // bamboo-core flags (spell check, ...)
    IBflags                uint                                // ibus-vhtime feature flags (xem bên dưới)
    Shortcuts              [10]uint32                          // 5 phím tắt, mỗi phím 2 uint32 (keyVal + mask)
    DefaultInputMode       int                                 // InputMode mặc định
    InputModeMapping       map[string]int                      // wmClass → InputMode override
}
```

**Lưu tại**: `~/.config/ibus-vhtime/ibus-vhtime.config.json`

**Default**:
- InputMethod: `"Telex"`
- OutputCharset: `"Unicode"`
- DefaultInputMode: `PreeditIM`
- IBflags: `IBstdFlags` (xem bên dưới)

## Input Mode Enum (config/flags.go)

```go
const (
    PreeditIM             = 1  // Gạch chân trong ô nhập (mặc định)
    SurroundingTextIM     = 2  // IBus SurroundingText API
    BackspaceForwardingIM = 3  // ForwardKeyEvent I
    ShiftLeftForwardingIM = 4  // ForwardKeyEvent II (Shift+Left)
    ForwardAsCommitIM     = 5  // Forward as commit
    XTestFakeKeyEventIM   = 6  // XTest fake key (cần libXtst)
    UsIM                  = 7  // Loại trừ (không gõ tiếng Việt)
)
```

Các mode từ 2–6 đều thuộc nhóm "backspace forwarding" (`ImBackspaceList`) — dùng khi app không hỗ trợ preedit.

## IBflags Bit Field

| Constant | Bit | Ý nghĩa |
|----------|-----|---------|
| `IBautoCommitWithVnNotMatch` | 0 | Auto-commit khi gõ không khớp từ Việt |
| `IBmacroEnabled` | 1 | Bật macro text |
| `IBspellCheckEnabled` | 4 | Bật kiểm tra chính tả |
| `IBautoNonVnRestore` | 5 | Khôi phục key gốc khi không phải từ Việt |
| `IBddFreeStyle` | 6 | Cho phép gõ dd tự do |
| `IBnoUnderline` | 7 | Tắt gạch chân preedit |
| `IBspellCheckWithRules` | 8 | Kiểm tra chính tả bằng quy tắc âm vị |
| `IBspellCheckWithDicts` | 9 | Kiểm tra chính tả bằng từ điển |
| `IBautoCommitWithDelay` | 10 | Auto-commit sau delay |
| `IBautoCommitWithMouseMovement` | 11 | Auto-commit khi chuột di chuyển |
| `IBpreeditElimination` | 13 | Xóa preedit trước khi gửi BS |
| `IBautoCapitalizeMacro` | 15 | Auto-capitalize macro |
| `IBmouseCapturing` | 18 | Bắt sự kiện chuột (X11) |
| `IBworkaroundForFBMessenger` | 19 | Workaround cho Facebook Messenger |
| `IBworkaroundForWPS` | 20 | Workaround cho WPS Office |

**IBstdFlags** (default bật): `IBspellCheckEnabled | IBspellCheckWithRules | IBautoNonVnRestore | IBddFreeStyle | IBmouseCapturing | IBautoCapitalizeMacro | IBnoUnderline | IBworkaroundForWPS`

## Keyboard Shortcuts

`Shortcuts [10]uint32` = 5 phím tắt, mỗi phím gồm `[keyVal, mask]`:

| Index | Phím tắt | Chức năng |
|-------|---------|---------|
| 0–1 | KSInputModeSwitch | Chuyển input mode |
| 2–3 | KSRestoreKeyStrokes | Khôi phục key gốc |
| 4–5 | KSViEnSwitch | Chuyển Việt/Anh |
| 6–7 | KSEmojiDialog | Mở emoji |
| 8–9 | KSHexadecimal | Nhập hex Unicode |

## IBus Property Keys (prop.go)

Các key xuất hiện trong IBus property panel (menu thanh công cụ):

```
PropKeyAbout, PropKeyStdToneStyle, PropKeyFreeToneMarking,
PropKeyEnableSpellCheck, PropKeySpellCheckByRules, PropKeySpellCheckByDicts,
PropKeyPreeditInvisibility, PropKeyVnCharsetConvert, PropKeyMouseCapturing,
PropKeyMacroEnabled, PropKeyMacroTable, PropKeyEmojiEnabled,
PropKeyConfiguration, PropKeyPreeditElimination, PropKeyInputModeLookupTable,
PropKeyAutoCapitalizeMacro
```

## Macro File

**Path**: `~/.config/ibus-vhtime/ibus-vhtime.macro.text`

**Format** (mỗi dòng): `keyword:replacement`

Comment: dòng bắt đầu bằng `;` hoặc `#`.

Template mẫu: `data/macro.tpl.txt`
