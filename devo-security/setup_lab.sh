#!/bin/bash
set -e

LAB_IMG="lab_disk.img"
MOUNT_POINT="/tmp/devos-lab"

echo "🔧 Setting up DevOS Security Lab..."

# 1. Создаем файл-диск (1GB)
if [ ! -f "$LAB_IMG" ]; then
    echo "Creating 1GB disk image..."
    dd if=/dev/zero of=$LAB_IMG bs=1M count=1024 status=progress
fi

# 2. Форматируем в Btrfs
echo "Formatting as Btrfs..."
mkfs.btrfs -f $LAB_IMG

# 3. Создаем точку монтирования
mkdir -p $MOUNT_POINT

# 4. Монтируем
echo "Mounting to $MOUNT_POINT..."
sudo mount -o loop $LAB_IMG $MOUNT_POINT

# 5. Меняем права (чтобы мы могли писать без sudo)
sudo chown $USER:$USER $MOUNT_POINT

echo "✅ Lab ready at $MOUNT_POINT"
echo "You can now run d-ransom against this directory."