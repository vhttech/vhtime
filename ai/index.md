# ibus-vhtime — AI Knowledge Index

> Tài liệu này là điểm khởi đầu để AI nắm bắt toàn bộ codebase mà không cần quét lại dự án.
> **Quy tắc bắt buộc**: Bất cứ khi nào có thay đổi ảnh hưởng đến kiến trúc, module, config,
> hoặc build pipeline — cập nhật file liên quan trong `ai/` ngay trong cùng PR/commit đó.

## Dự án là gì?

**ibus-vhtime** là một bộ gõ tiếng Việt (Vietnamese Input Method Editor) chạy trên IBus framework,
viết bằng Go + CGo. Phiên bản hiện tại: **1.0.0**.

- Hỗ trợ Linux (X11 + Wayland) và FreeBSD
- Phương thức gõ: Telex, VNI, VIQR, … (bamboo-core xử lý)
- Bộ ký tự đầu ra: Unicode, TCVN3, VNI, …
- Tích hợp: emoji, macro text, từ điển chính tả

## Cây thư mục chính

```
ibus-vhtime/
├── main.go                 # Entry point, khởi tạo IBus bus + engine factory
├── engine.go               # IBusBambooEngine struct + ProcessKeyEvent dispatcher
├── engine_preedit.go       # Mode: Preedit (gạch chân trong ô nhập)
├── engine_backspace.go     # Mode: Backspace forwarding (sửa lỗi gạch chân)
├── engine_emoji.go         # Bảng chọn emoji
├── engine_hexadecimal.go   # Nhập ký tự hex
├── engine_utils.go         # Helper: commit text, reset, detect window class
├── prop.go                 # IBus property panel (menu thanh công cụ)
├── client.go               # Wayland toplevel protocol (auto-gen)
├── wl_introspector.go      # Lấy window class trên Wayland
├── gnome_introspector.go   # Lấy window class trên GNOME/DBus
├── x11.go                  # CGo bridge: clipboard, mouse capture, fake key
├── x11_clipboard.c         # X11 clipboard copy/paste
├── x11_introspector.c      # Lấy focus window class (X11)
├── x11_keyboard.c          # Gửi backspace/shift qua XTest
├── x11_mouse.c             # Mouse capture (phát hiện click chuột)
├── x11_record.c            # X11 Record extension (keypress capturing)
├── mactab.go               # Macro table (từ viết tắt → chuỗi đầy đủ)
├── trie.go                 # Trie data structure (cho emoji lookup)
├── emoji.go                # Load + tìm kiếm emoji
├── utils.go                # Constants, helper functions, dict loading
├── ibus_const.go           # IBus key masks + capability constants
├── fake_engine.go          # IEngine interface + FakeEngine (testing)
├── version.go              # var Version string (inject lúc build)
├── config/
│   ├── config.go           # Config struct, load/save JSON
│   └── flags.go            # Input mode enum + IBflags bit constants
├── ui/
│   ├── ui.go               # CGo bridge → GTK3 GUI (openGUI)
│   └── keyboard-shortcut-editor.c  # Widget C cho cài phím tắt
├── data/                   # vhtime.xml, từ điển, emoji JSON, desktop file
├── scripts/
│   ├── build               # Shell script build Go binary
│   ├── install             # Shell script cài vào PREFIX
│   └── test                # Shell script chạy go test
├── build/                  # PKGBUILD (Arch), spec (RPM), deb packaging
├── tests/                  # xtest.c (X11 integration test)
└── vendor/                 # Vendored deps (go mod vendor)
```

## File tài liệu trong `ai/`

| File | Nội dung |
|------|----------|
| [architecture.md](architecture.md) | Module breakdown, data flow, các type chính |
| [engine.md](engine.md) | Pipeline xử lý phím, các input mode, IBusBambooEngine |
| [config.md](config.md) | Config struct, IBflags bit field, input mode enum |
| [build.md](build.md) | Build system, CI/CD, packaging |

## Tech Stack nhanh

| Thành phần | Chi tiết |
|-----------|---------|
| Ngôn ngữ | Go 1.13+ với CGo |
| IME framework | IBus (via goibus) |
| NLP core | bamboo-core (xử lý âm vị tiếng Việt) |
| GUI | GTK3 (CGo) |
| X11 | libX11, libXtst (clipboard, mouse, fake key) |
| Wayland | wlr-foreign-toplevel protocol |
| D-Bus | godbus/dbus v5 |
| Build | Make + shell scripts |
| CI | GitHub Actions (Linux + FreeBSD VM + Nix flake) |
