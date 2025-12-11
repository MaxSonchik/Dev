#!/bin/bash
set -e

# 1. Выбор движка
if command -v podman &> /dev/null; then
    CONTAINER_ENGINE="podman"
    echo "✅ Обнаружен Podman."
elif command -v docker &> /dev/null; then
    CONTAINER_ENGINE="docker"
    if ! systemctl is-active --quiet docker; then
        echo "❌ Docker не запущен."
        exit 1
    fi
    echo "✅ Обнаружен Docker."
else
    echo "❌ Нет контейнерного движка."
    exit 1
fi

IMAGE_NAME="archlinux:latest"
WORK_DIR_LOCAL="$(pwd)"
WORK_DIR_CONTAINER="/devos"

# 2. Генерация стандартного pacman.conf (Без сторонних репо)
echo "⚙️  Сброс конфигурации pacman на стандартную..."
cat > iso-profile/pacman.conf <<EOF
[options]
HoldPkg     = pacman glibc
Architecture = auto
ParallelDownloads = 5
SigLevel    = Required DatabaseOptional
LocalFileSigLevel = Optional

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist
EOF

echo "🐳 [DevOS Wrapper] Запуск сборки..."

# 3. Запуск
sudo $CONTAINER_ENGINE run --rm --privileged --network host \
    -v "$WORK_DIR_LOCAL":"$WORK_DIR_CONTAINER" \
    -w "$WORK_DIR_CONTAINER" \
    "$IMAGE_NAME" \
    /bin/bash -c "
        # Инициализация
        echo '📦 [Container] Init keys...'
        pacman-key --init
        pacman-key --populate archlinux
        pacman -Sy --noconfirm archlinux-keyring

        # Установка тулчейна
        echo '📦 [Container] Install build tools...'
        pacman -S --noconfirm archiso git make

        # Очистка
        echo '🧹 [Container] Cleaning workspace...'
        rm -rf work/*

        # Сборка
        echo '🚀 [Container] Building ISO...'
        mkarchiso -v -w /devos/work -o /devos/out /devos/iso-profile
    "

echo "✅ [DevOS Wrapper] Сборка завершена! Проверяйте папку out/"