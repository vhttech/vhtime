# Build, CI/CD & Packaging

## Build

```bash
make build       # Chạy scripts/build
make test        # Chạy scripts/test (go test ./...)
make install     # Build + scripts/install PREFIX DESTDIR
make clean
make src         # Tạo tarball nguồn
```

**scripts/build**:
- Linux: `CGO_ENABLED=1 go build -o ibus-engine-vhtime -ldflags "-w -s -X main.Version=0.8.4" -mod=vendor`
- FreeBSD: thêm `CGO_CFLAGS=-I/usr/local/include`, `CGO_LDFLAGS=-L/usr/local/lib`

**Version**: inject lúc build qua `-X main.Version`, biến khai báo trong `version.go`.

## Phụ thuộc build

| Thư viện | Package (Debian) | Dùng cho |
|---------|-----------------|---------|
| libibus-1.0-dev | ibus-dev | IBus D-Bus protocol |
| libx11-dev | - | X11 display |
| libxtst-dev | - | XTest (fake key events) |
| libgtk-3-dev | - | GTK3 GUI |

## CI/CD (`.github/workflows/release.yaml`)

3 jobs chạy song song khi push:

### 1. Build-test-freebsd
- Dùng `vmactions/freebsd-vm@v1` (FreeBSD 15.0)
- Cài: `go pkgconf libX11 libXtst gtk3`
- Chạy: `make test && make build`

### 2. Build-test-nix-flake
- Dùng `cachix/install-nix-action@v27`
- Build: `nix build` + `nix develop`
- Kiểm tra Nix flake (`flake.nix` + `flake.lock`)

### 3. Releaser (Linux)
- Ubuntu với apt: `libibus-1.0-dev libx11-dev libxtst-dev libgtk-3-dev osc`
- `make test && make build`
- Sau đó chạy `scripts/osc.bash` để publish lên OpenSUSE Build Service (OBS)
- Cần secrets: `OSC_USER`, `OSC_PASS`, `OSC_PATH`, `GH_TAG`

## Packaging

| Format | Thư mục | Ghi chú |
|--------|---------|---------|
| Arch Linux | `build/arch/` | PKGBUILD-git, PKGBUILD-obs, PKGBUILD-release |
| RPM | `build/rpm/ibus-vhtime.spec` | rpmbuild |
| DEB | `build/deb/` | dpkg-buildpackage |
| OBS | `scripts/osc.bash` | OpenSUSE Build Service |
| Nix | `flake.nix` + `flake.lock` | Nix flake |

## Cài đặt thủ công

```bash
make install PREFIX=/usr DESTDIR=/tmp/pkg
```

**scripts/install** đặt file vào:
- `$PREFIX/share/ibus-vhtime/` — data files (vhtime.xml, dict, emoji, desktop)
- `$PREFIX/lib/ibus-vhtime/` — binary `ibus-engine-vhtime`
- `$PREFIX/share/ibus/component/vhtime.xml` — đăng ký engine với IBus

## Go Modules

Dự án dùng Go modules chuẩn (không vendor). Dependencies được khai báo trong `go.mod` và khóa hash tại `go.sum`. Go tự tải về khi build.

```
github.com/BambooEngine/bamboo-core   # Vietnamese NLP
github.com/BambooEngine/goibus        # IBus Go binding
github.com/dkolbly/wl                 # Wayland client
github.com/godbus/dbus/v5             # D-Bus
golang.org/x/net                      # context package
```

Để cập nhật dependencies: `go mod tidy`
