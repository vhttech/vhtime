vhtime - Vietnamese IME for Linux/BSD
===================================

## Maintainer

**Fx Phúc Vinh** — [vinhhp@vhttech.com](mailto:vinhhp@vhttech.com)

Forked from [ibus-bamboo](https://github.com/BambooEngine/ibus-bamboo) (original author: Luong Thanh Lam).

## Build from source

### Ubuntu / Debian

```sh
sudo apt-get install -y golang libibus-1.0-dev libx11-dev libxtst-dev libgtk-3-dev
make build
sudo make install PREFIX=/usr
ibus restart
```

### Fedora / RHEL

```sh
sudo dnf install -y golang ibus-devel libX11-devel libXtst-devel gtk3-devel
make build
sudo make install PREFIX=/usr
ibus restart
```

### Arch Linux

```sh
sudo pacman -S go ibus libx11 libxtst gtk3
make build
sudo make install PREFIX=/usr
ibus restart
```

## License

vhtime is released under the GNU General Public License v3.0.
