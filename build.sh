#!/bin/bash
set -e

# Конфигурация
PROFILE_DIR="iso-profile"
WORK_DIR="work"
OUT_DIR="out"

echo "🚀 [DevOS Builder] Starting build process..."

# Очистка предыдущей сборки (если нужно)
if [ "$1" == "--clean" ]; then
    echo "🧹 Cleaning up work directories..."
    sudo rm -rf $WORK_DIR $OUT_DIR
fi

mkdir -p $OUT_DIR

# Запуск сборки
# -v: verbose output
# -w: work directory
# -o: output directory
# profile_dir: путь к нашему профилю
sudo mkarchiso -v -w "$WORK_DIR" -o "$OUT_DIR" "$PROFILE_DIR"

echo "✅ [DevOS Builder] Build complete!"
echo "📂 ISO is located in: $OUT_DIR"
