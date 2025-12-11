#!/bin/bash

# Находим ISO
ISO=$(ls out/*.iso | head -n 1)

if [ -z "$ISO" ]; then
    echo "❌ ISO не найден! Сначала выполните ./build.sh"
    exit 1
fi

echo "🚀 Запуск $ISO..."

# В Fedora пути к OVMF (UEFI) специфичные.
# Мы ищем именно версию БЕЗ Secure Boot (OVMF_CODE.fd)
OVMF_CODE="/usr/share/edk2/ovmf/OVMF_CODE.fd"
OVMF_VARS="/usr/share/edk2/ovmf/OVMF_VARS.fd"

# Создаем временные переменные, чтобы сбрасывать настройки BIOS при каждом запуске
cp "$OVMF_VARS" /tmp/my_vars.fd

qemu-system-x86_64 \
    -enable-kvm \
    -m 4G \
    -smp 2 \
    -cpu host \
    -drive if=pflash,format=raw,readonly=on,file="$OVMF_CODE" \
    -drive if=pflash,format=raw,file=/tmp/my_vars.fd \
    -cdrom "$ISO" \
    -vga virtio \
    -display gtk,gl=on \
    -device intel-hda -device hda-duplex \
    -usb -device usb-tablet