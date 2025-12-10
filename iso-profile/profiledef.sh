#!/usr/bin/env bash
# iso-profile/profiledef.sh

iso_name="devos"
iso_label="DEVOS_$(date +%Y%m)"
iso_publisher="DevOS Team <https://devos.io>"
iso_application="DevOS Live/Rescue CD"
iso_version="$(date +%Y.%m.%d)"
install_dir="devos"
buildmodes=('iso')
bootmodes=('bios.syslinux.mbr' 'bios.syslinux.eltorito'
           'uefi-x64.systemd-boot.esp' 'uefi-x64.systemd-boot.eltorito')
arch="x86_64"
pacman_conf="pacman.conf"
airootfs_image_type="squashfs"
airootfs_image_tool_options=('-comp' 'zstd')
file_permissions=(
  ["/etc/shadow"]="0:0:400"
  ["/root"]="0:0:750"
  ["/root/.automated_script.sh"]="0:0:755"
  ["/usr/local/bin/choose-mirror"]="0:0:755"
  ["/usr/local/bin/Installation_guide"]="0:0:755"
)
