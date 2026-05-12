Checklist debug (chạy theo thứ tự)

1. Kiểm tra IBus daemon có đang chạy không:
pgrep -a ibus-daemon
ibus version

2. IBus có thấy engine vhtime không:
ibus list-engine | grep -i vhtime
Nếu không có output → IBus chưa đọc component XML.

3. Reload component:
ibus-daemon -drx   # restart daemon với --daemonize --replace --xim
ibus list-engine | grep -i vhtime

4. Nếu vẫn không thấy, kiểm tra XML có hợp lệ không:
ibus write-cache --system   # rebuild ibus component cache (cần sudo)
# hoặc:
sudo ibus write-cache

5. Thêm engine vào IBus:
ibus-setup   # mở IBus Preferences → Input Method → Add → Vietnamese (vhtime)
Hoặc dùng gsettings (GNOME):
gsettings get org.gnome.desktop.input-sources sources
gsettings set org.gnome.desktop.input-sources sources "[('xkb','us'),('ibus','Vhtime')]"

Kiểm tra nhanh nhất

Lệnh này nói rõ nhất vấn đề:
ibus list-engine 2>&1 | grep -i -A3 vhtime
- Thấy "Vhtime" → engine đã đăng ký, vấn đề nằm ở GNOME Settings hoặc ibus-setup chưa add
- Không thấy gì → IBus chưa đọc XML, cần ibus write-cache + restart daemon