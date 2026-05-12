#!/bin/bash
# ibus-vhtime installer — GUI (zenity) with terminal fallback

VERSION="0.8.4"
PKG_NAME="ibus-vhtime"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TITLE="Cài đặt $PKG_NAME $VERSION"

# Detect GUI availability
USE_GUI=false
if [ -n "${DISPLAY}${WAYLAND_DISPLAY}" ] && command -v zenity >/dev/null 2>&1; then
    USE_GUI=true
fi

# ── Helpers ─────────────────────────────────────────────────────────────
z() { zenity --title="$TITLE" --width=420 "$@" 2>/dev/null; }

say()  { $USE_GUI && z --info  --text="$1" --ok-label="Đóng"   || echo -e "$1"; }
fail() { $USE_GUI && z --error --text="$1"                      || echo -e "Lỗi: $1" >&2; exit 1; }

# ── Step 1: Welcome ──────────────────────────────────────────────────────
if $USE_GUI; then
    z --question \
        --text="<big><b>ibus-vhtime $VERSION</b></big>\nBộ gõ tiếng Việt cho Linux\n\nSẽ cài vào:\n  <tt>/usr/lib/ibus-vhtime/</tt>\n  <tt>/usr/share/ibus-vhtime/</tt>\n  <tt>/usr/share/ibus/component/</tt>" \
        --ok-label="  Cài đặt  " --cancel-label="Hủy" || exit 0
else
    printf "\033[1m==> $TITLE\033[0m\n"
    printf "Tiếp tục? [Y/n] " && read -r _ans
    case "$_ans" in [Nn]*) exit 0;; esac
fi

# ── Step 2: Write install script to temp file ─────────────────────────────
TMP=$(mktemp /tmp/vhtime-install.XXXXXX)
cat > "$TMP" << EOF
#!/bin/sh
set -e
mkdir -p /usr/lib/ibus-vhtime /usr/share/ibus/component /usr/share/applications
cp -rf "${SCRIPT_DIR}/usr/lib/ibus-vhtime/." /usr/lib/ibus-vhtime/
cp -rf "${SCRIPT_DIR}/usr/share/ibus-vhtime"  /usr/share/
cp -f  "${SCRIPT_DIR}/usr/share/ibus/component/vhtime.xml" /usr/share/ibus/component/
cp -f  "${SCRIPT_DIR}/usr/share/applications/ibus-setup-vhtime.desktop" /usr/share/applications/
chmod +x /usr/lib/ibus-vhtime/ibus-engine-vhtime
EOF
chmod +x "$TMP"

# ── Step 3: Run as root (pkexec shows polkit dialog; sudo for terminal) ───
if $USE_GUI; then
    pkexec "$TMP" 2>/dev/null
    RC=$?
else
    sudo "$TMP"
    RC=$?
fi
rm -f "$TMP"

[ $RC -ne 0 ] && fail "Cài đặt thất bại.\nChạy lại trong terminal:\n  <tt>sudo bash install.sh</tt>"

# ── Step 4: Restart IBus ─────────────────────────────────────────────────
ibus restart 2>/dev/null || true

# ── Step 5: Done ─────────────────────────────────────────────────────────
if $USE_GUI; then
    z --info \
        --text="<big>✓ Cài đặt thành công!</big>\n\n<b>Thêm bộ gõ:</b>\nSettings → Keyboard → Input Sources\n→ nhấn <b>+</b> → tìm <b>Vietnamese (vhtime)</b>" \
        --ok-label="Đóng" 2>/dev/null
else
    printf "\033[32m✓ Cài đặt thành công!\033[0m\n"
    echo ""
    echo "Thêm bộ gõ: Settings → Keyboard → Input Sources → + → Vietnamese (vhtime)"
fi
