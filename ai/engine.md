# Engine — Xử lý phím và Input Pipeline

## IBusBambooEngine struct (engine.go)

```go
type IBusBambooEngine struct {
    sync.Mutex
    IEngine                              // IBus base engine (goibus)
    preeditor              bamboo.IEngine // Xử lý âm vị (bamboo-core)
    engineName             string
    config                 *config.Config
    propList               *ibus.PropList
    englishMode            bool           // Tắt/bật chế độ tiếng Việt
    macroTable             *MacroTable
    wmClasses              string         // Class cửa sổ đang focus
    isInputModeLTOpened    bool
    isEmojiLTOpened        bool
    isInHexadecimal        bool
    emojiLookupTable       *ibus.LookupTable
    inputModeLookupTable   *ibus.LookupTable
    capabilities           uint32
    nFakeBackSpace         int32          // Số BS fake đang gửi
    emoji                  *EmojiEngine
    isSurroundingTextReady bool
    shouldRestoreKeyStrokes bool
    shouldEnqueuKeyStrokes  bool
}
```

## ProcessKeyEvent — dispatcher chính

Thứ tự kiểm tra trong `ProcessKeyEvent()`:

1. **Release event** → bỏ qua (chỉ xử lý press)
2. **Modifier-only key** (Ctrl, Alt, Super) → bỏ qua
3. **Hexadecimal mode** đang bật → `hexProcessKeyEvent()`
4. **Emoji lookup table** đang mở → `emojiProcessKeyEvent()`
5. **Input mode lookup table** đang mở → xử lý chọn mode
6. **English mode** hoặc window trong exclusion list → pass through
7. **Shortcut keys** (keyboard shortcuts từ config) → xử lý
8. Dispatch theo `config.DefaultInputMode` (per-window override nếu có)

## Preedit Mode (engine_preedit.go)

Chế độ mặc định — hiện text gạch chân trong ô nhập (underline preedit).

**Flow**:
```
KeyPress → preeditor.ProcessKey() → getPreeditString() → UpdatePreedit()
                                                              │
                                     (khi xong từ/commit) → commitPreeditAndReset()
```

**Đặc điểm**:
- Backspace xóa ký tự cuối trong preeditor (không gửi BS thật)
- Tab khi có preedit → commit ngay
- Phím di chuyển (arrow, Home, End) → commit và pass through
- Workaround cho Chrome address bar: không xử lý ký tự đặc biệt khi buffer rỗng

## Backspace Forwarding Mode (engine_backspace.go)

Dùng cho các app không hỗ trợ preedit tốt (VS Code, terminal, …).

**Cơ chế**:
1. Gõ ký tự → bamboo-core tính string mới
2. So sánh với string cũ → tính số ký tự cần xóa
3. Gửi N backspace (thật hoặc fake qua XTest)
4. Commit string mới

**Các variant**:
- `BackspaceForwardingIM`: ForwardKeyEvent (IBus)
- `ShiftLeftForwardingIM`: Shift+Left để xóa
- `ForwardAsCommitIM`: commit từng ký tự
- `XTestFakeKeyEventIM`: XTest extension (x11_keyboard.c)
- `SurroundingTextIM`: dùng IBus SurroundingText API

**Key queue**: `shouldEnqueuKeyStrokes` — key events được enqueue khi đang gửi BS để tránh race condition.

## Emoji Mode (engine_emoji.go)

- Trigger: gõ `:` ở đầu từ
- Hiện `LookupTable` với danh sách emoji khớp tên
- Dùng Trie để tìm kiếm nhanh trong `emojione.json`
- Xác nhận: Enter / số thứ tự; Hủy: Escape

## Hexadecimal Mode (engine_hexadecimal.go)

- Trigger: Ctrl+Shift+U (theo chuẩn GTK)
- Gõ mã hex Unicode → Enter để commit ký tự
- Ví dụ: `1ED3` → `ổ`

## Window Class & Per-app Input Mode

`wmClasses` được cập nhật liên tục bởi goroutine nền (X11/Wayland/GNOME).

`config.InputModeMapping` map `wmClass → inputMode` — user có thể cài riêng cho từng app.

Nếu app trong `UsIM` list → engine bypass hoàn toàn (không gõ tiếng Việt).

## Key Press Capturing (x11_record.c)

`keyPressCapturing()` dùng X11 Record extension để bắt global key events:
- Phát hiện click chuột → `commitAndReset()` (tránh lỗi khi user click giữa chừng gõ)
- Phát hiện mouse movement (nếu `IBmouseCapturing` bật) → auto-commit

## Goroutines trong engine

| Goroutine | Chạy ở | Mục đích |
|-----------|--------|---------|
| `keyPressCapturing()` | main/prop.go | Global key/mouse monitor |
| `wlGetFocusWindowClass()` | main.go | Wayland window tracker |
| `engine.init()` | prop.go | Khởi tạo async (load dict, macro) |
