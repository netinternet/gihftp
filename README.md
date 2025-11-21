# GIH-FTP

GIH Güvenli İnternet Yazılımı ile iletişime geçerek birden fazla DNS sunucusundan log dosyalarını alan, birleştiren, istek sayısına göre sıralayan ve BTK FTP sunucusuna yükleyen bir Go uygulamasıdır.

## Özellikler

- ✅ **Command-line flag desteği** - Binary olarak flag'lerle çalışır
- ✅ **Backward compatible** - Eski config dosyası formatını destekler
- ✅ **Güvenli SFTP** - SSH key authentication ve host key verification
- ✅ **TLS güvenliği** - Sertifika doğrulama desteği
- ✅ **Structured logging** - Debug/Info/Error seviyeleri ile detaylı loglama
- ✅ **Çalışma dizini yönetimi** - Geçici dosyalar için özel dizin
- ✅ **Hata yönetimi** - Partial success desteği, exit code'lar
- ✅ **Modüler yapı** - Paketlere ayrılmış temiz kod

## Kurulum

### Ön-derlenmiş Binary ile (Önerilen)

1. **Release paketini indirin:**
   - GitHub Releases sayfasından platformunuza uygun `.tar.gz` dosyasını indirin
   - Örnek: `gihftp-v2.0.0-linux-amd64.tar.gz`

2. **Paketi açın:**
   ```bash
   tar -xzf gihftp-v2.0.0-linux-amd64.tar.gz
   cd gihftp-v2.0.0-linux-amd64/
   ```

3. **Kurulum scripti ile yükleyin:**
   ```bash
   sudo ./install.sh
   ```

4. **Yapılandırın:**
   ```bash
   sudo nano /etc/gihftp.conf
   ```

5. **Test edin:**
   ```bash
   gihftp --help
   ```

### Kaynak Koddan Derleme (Geliştirici)

**Not:** Production ortamlarında kaynak koddan derleme yerine pre-built binary kullanın.

```bash
# Basit build
go build -o gihftp

# Release build (optimized)
go build -ldflags="-s -w" -o gihftp
```

### Release Paketi Oluşturma (Maintainer)

Tüm platformlar için release paketi oluşturmak:

```bash
./make-release.sh v2.0.0
```

Bu komut şunları oluşturur:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Checksum dosyaları
- Kurulum scriptleri
- Dokümantasyon

**Önemli:** Release paketleri sadece binary içerir, kaynak kod içermez!

## Kullanım

### 1. Command-line Flags ile (Önerilen)

```bash
./gihftp \
  --gih-servers=dns1.example.com,dns2.example.com \
  --gih-api-port=2035 \
  --ftp-host=ftp.btk.gov.tr \
  --ftp-user=btk_user \
  --ftp-log-dir=/var/log/gih/ \
  --ssh-key=/root/.ssh/id_rsa \
  --work-dir=/tmp/gihftp \
  --log-level=info \
  --cleanup
```

Password'ü environment variable ile geçirin (güvenlik için):
```bash
export FTP_PASSWORD="your_secure_password"
./gihftp --gih-servers=... --ftp-host=...
```

### 2. Config Dosyası ile (Backward Compatible)

```bash
# /etc/gihftp.conf veya ./gihftp.conf kullanarak
./gihftp

# veya custom config dosyası ile
./gihftp --config=/path/to/custom.conf
```

Config dosyası formatı (`gihftp.conf.example` dosyasına bakın):
```ini
# GIH Public DNS 1
gihdns1 = dns1.example.com

# GIH Public DNS 2
gihdns2 = dns2.example.com

# GIH API Port
gihapiport = 2035

# SFTP Server
ftpserver = ftp.btk.gov.tr

# SFTP Server log files directory
ftplogdir = /var/log/gih/
```

## Flag Parametreleri

| Flag | Açıklama | Default | Zorunlu |
|------|----------|---------|---------|
| `--gih-servers` | Virgülle ayrılmış GIH DNS sunucu adresleri | - | ✅ |
| `--gih-api-port` | GIH API port numarası | 2035 | ❌ |
| `--ftp-host` | SFTP sunucu adresi | - | ✅ |
| `--ftp-user` | SFTP kullanıcı adı | root | ❌ |
| `--ftp-password` | SFTP şifresi (env var tercih edilir) | - | ❌ |
| `--ftp-log-dir` | Uzak sunucuda log dizini | /var/log/gih/ | ❌ |
| `--ssh-key` | SSH private key path | $HOME/.ssh/id_rsa | ❌ |
| `--work-dir` | Geçici dosyalar için çalışma dizini | . (mevcut dizin) | ❌ |
| `--log-level` | Log seviyesi (debug/info/error) | info | ❌ |
| `--cleanup` | Upload sonrası geçici dosyaları sil | true | ❌ |
| `--insecure-skip-verify` | TLS/SSH doğrulamayı atla (ÖNERİLMEZ!) | false | ❌ |
| `--config` | Config dosyası path | - | ❌ |

## Environment Variables

| Variable | Açıklama |
|----------|----------|
| `FTP_PASSWORD` | SFTP şifresi (flag'den daha güvenli) |
| `SSH_KEY_PASSPHRASE` | SSH key şifresi (eğer key şifreliyse) |

## Güvenlik

### ✅ Güvenli Kullanım (Önerilen)

```bash
# 1. SSH key authentication kullanın
./gihftp \
  --gih-servers=dns1.example.com \
  --ftp-host=ftp.btk.gov.tr \
  --ssh-key=/root/.ssh/id_rsa

# 2. Password'ü environment variable ile geçirin
export FTP_PASSWORD="secure_password"
./gihftp --gih-servers=dns1.example.com --ftp-host=ftp.btk.gov.tr

# 3. TLS/SSH doğrulamasını aktif bırakın (default)
# known_hosts dosyanızı güncel tutun
```

### ⚠️ Güvensiz Kullanım (Sadece Test İçin)

```bash
# Bu sadece test ortamları için! Production'da kullanmayın!
./gihftp \
  --gih-servers=test-dns.local \
  --ftp-host=test-ftp.local \
  --insecure-skip-verify
```

## Çıkış Kodları

Uygulama aşağıdaki exit code'ları döner:

| Kod | Anlamı |
|-----|--------|
| 0 | Başarılı |
| 1 | Konfigürasyon hatası |
| 2 | Log fetch hatası (hiçbir sunucudan veri alınamadı) |
| 3 | Merge hatası |
| 4 | Upload hatası |
| 5 | Kısmi başarı (bazı sunuculardan veri alınamadı ama işlem tamamlandı) |

## Loglama

### Log Seviyeleri

```bash
# Debug - Her detayı göster
./gihftp --log-level=debug ...

# Info - Normal işlem logları (default)
./gihftp --log-level=info ...

# Error - Sadece hataları göster
./gihftp --log-level=error ...
```

### Örnek Log Çıktısı

```
time=2025-01-20T10:30:00.000Z level=INFO msg="GIH-FTP Service Starting" version=2.0.0 gih_servers="[dns1.example.com dns2.example.com]" ftp_host=ftp.btk.gov.tr work_dir=/tmp/gihftp
time=2025-01-20T10:30:00.100Z level=INFO msg="Fetching logs for date range" start_date=20250113 end_date=20250119
time=2025-01-20T10:30:01.250Z level=INFO msg="Fetched log files" host=dns1.example.com count=7
time=2025-01-20T10:30:05.500Z level=INFO msg="Merge statistics" unique_domains=12543 total_requests=1523442 top_domain=google.com top_domain_hits=45231
time=2025-01-20T10:30:05.750Z level=INFO msg="SFTP upload completed" bytes_uploaded=524288 duration_seconds=0.5 speed_mbps=1.00
time=2025-01-20T10:30:05.800Z level=INFO msg="GIH-FTP Service completed successfully"
```

## Proje Yapısı

### Kaynak Kod Yapısı (Geliştirici İçin)

```
gih-ftp/
├── main.go                      # Ana program
├── internal/
│   ├── config/                  # Konfigürasyon yönetimi
│   │   └── config.go
│   ├── gihapi/                  # GIH API client
│   │   └── client.go
│   ├── sftp/                    # SFTP upload işlemleri
│   │   └── client.go
│   ├── merger/                  # Log merge işlemleri
│   │   └── merger.go
│   └── logger/                  # Loglama
│       └── logger.go
├── gihftp.conf.example          # Örnek konfig dosyası
├── make-release.sh              # Release builder
├── install.sh                   # Kurulum scripti
├── uninstall.sh                 # Kaldırma scripti
├── deploy.sh                    # Development deployment
├── .gitignore                   # Git ignore kuralları
├── go.mod
├── go.sum
└── README.md
```

### Release Paketi Yapısı (Son Kullanıcı İçin)

```
gihftp-v2.0.0-linux-amd64/
├── gihftp                       # Binary (SADECE)
├── gihftp.sha256                # Checksum
├── README.md                    # Dokümantasyon
├── INSTALL.txt                  # Kurulum rehberi
├── CHANGELOG.md                 # Değişiklik listesi
├── LICENSE                      # Lisans
├── gihftp.conf.example          # Örnek konfig
├── install.sh                   # Otomatik kurulum
└── uninstall.sh                 # Otomatik kaldırma
```

**Önemli:** Release paketlerinde kaynak kod yoktur!

## Deployment

### Ön-derlenmiş Binary ile Deployment (Önerilen)

```bash
# 1. Release paketi oluştur (maintainer)
./make-release.sh v2.0.0

# 2. Release paketini sunucuya kopyala
scp releases/v2.0.0/gihftp-v2.0.0-linux-amd64.tar.gz root@server:/tmp/

# 3. Sunucuda paketi aç ve kur
ssh root@server
cd /tmp
tar -xzf gihftp-v2.0.0-linux-amd64.tar.gz
cd gihftp-v2.0.0-linux-amd64/
sudo ./install.sh
```

### Manuel Deployment (Eski Yöntem)

```bash
# Build
GOOS=linux GOARCH=amd64 go build -o gihftp

# Server'a kopyala
scp gihftp root@your-server.example.com:/usr/bin/
scp gihftp.conf root@your-server.example.com:/etc/gihftp.conf

# Çalıştırılabilir yap
ssh root@your-server.example.com "chmod +x /usr/bin/gihftp"
```

### Development Deployment Script

**Not:** Bu script sadece geliştirme amaçlıdır. Production için `make-release.sh` kullanın.

```bash
./deploy.sh  # Kaynak koddan derleyip deploy eder
```

### Cron ile Otomatik Çalıştırma

Haftalık otomatik çalıştırma için crontab'e ekleyin:

```bash
# Her pazartesi sabah 3:00'da çalıştır
0 3 * * 1 /usr/bin/gihftp --config=/etc/gihftp.conf >> /var/log/gihftp.log 2>&1

# veya flag ile
0 3 * * 1 FTP_PASSWORD="xxx" /usr/bin/gihftp --gih-servers=dns1,dns2 --ftp-host=ftp.btk.gov.tr >> /var/log/gihftp.log 2>&1
```

## Systemd Service (Opsiyonel)

`/etc/systemd/system/gihftp.service`:

```ini
[Unit]
Description=GIH FTP Log Upload Service
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/bin/gihftp --config=/etc/gihftp.conf
Environment="FTP_PASSWORD=your_password"
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Kullanım:
```bash
# Manuel çalıştırma
sudo systemctl start gihftp

# Status kontrolü
sudo systemctl status gihftp

# Otomatik başlatma
sudo systemctl enable gihftp
```

## Troubleshooting

### Problem: "No GIH servers specified"
**Çözüm:** `--gih-servers` flag'ini veya config dosyasında `gihdns1`, `gihdns2` değerlerini belirtin.

### Problem: "SSH connection failed"
**Çözüm:**
- SSH key path'ini kontrol edin (`--ssh-key`)
- Key'in passphrase'i varsa `SSH_KEY_PASSPHRASE` env var'ını set edin
- Veya password authentication kullanın (`FTP_PASSWORD` env var)

### Problem: "Remote host identification has changed"
**Çözüm:**
- `~/.ssh/known_hosts` dosyasını güncelleyin
- Veya test için `--insecure-skip-verify` kullanın (güvensiz!)

### Problem: TLS certificate verification failed
**Çözüm:**
- GIH sunucularının sertifikalarının geçerli olduğundan emin olun
- Self-signed sertifika kullanıyorsanız test için `--insecure-skip-verify` kullanabilirsiniz

### Debug Mode

Detaylı log için:
```bash
./gihftp --log-level=debug --gih-servers=... --ftp-host=...
```

## Release Yönetimi

### Release Paketi Oluşturma (Maintainer için)

1. **Kodu test edin:**
   ```bash
   go test ./...
   go build -o gihftp
   ./gihftp --help
   ```

2. **Release oluşturun:**
   ```bash
   ./make-release.sh v2.0.0
   ```

3. **Checksum'ları doğrulayın:**
   ```bash
   cd releases/v2.0.0/
   sha256sum -c *.sha256
   ```

4. **Paketi test edin:**
   ```bash
   tar -xzf gihftp-v2.0.0-linux-amd64.tar.gz
   cd gihftp-v2.0.0-linux-amd64/
   ./gihftp --help
   ```

5. **Dağıtın:**
   - Release paketlerini GitHub Releases'e yükleyin
   - Checksum dosyalarını da ekleyin
   - Release notes'u CHANGELOG.md'den alın

### Güvenli Dağıtım Politikası

**ASLA kaynak kod dağıtmayın!** Sadece derlenmiş binary'leri dağıtın:

✅ **İzin verilen:**
- Binary dosyalar (gihftp, gihftp.exe)
- Dokümantasyon (README, INSTALL.txt, CHANGELOG)
- Kurulum scriptleri (install.sh, uninstall.sh)
- Örnek konfig dosyası (gihftp.conf.example)
- Checksum dosyaları (*.sha256)

❌ **İzin verilmeyen:**
- Kaynak kod dosyaları (*.go)
- go.mod, go.sum
- internal/ dizini
- .git dizini
- Geliştirme scriptleri (make-release.sh, deploy.sh)

### Checksum Doğrulama (Son Kullanıcı için)

Binary'nin bütünlüğünü doğrulayın:

```bash
# SHA256 checksum kontrolü
sha256sum gihftp
cat gihftp.sha256

# Eşleşmeli!
```

## Katkıda Bulunma

1. Fork edin
2. Feature branch oluşturun (`git checkout -b feature/amazing-feature`)
3. Commit edin (`git commit -m 'feat: Add amazing feature'`)
4. Test edin (`go test ./...`)
5. Push edin (`git push origin feature/amazing-feature`)
6. Pull Request açın

**Not:** Pull request'ler için:
- Kod stil standartlarına uyun
- Test ekleyin
- README'yi güncelleyin
- Breaking change'leri belirtin

## Lisans

Bu proje NI (Netinternet) için geliştirilmiştir.

## Değişiklikler (v2.0.0)

### Yeni Özellikler
- ✅ Command-line flag desteği
- ✅ Environment variable desteği (FTP_PASSWORD)
- ✅ Çalışma dizini yönetimi (--work-dir)
- ✅ Structured logging (debug/info/error)
- ✅ SSH host key verification
- ✅ TLS certificate verification
- ✅ Exit code standardizasyonu
- ✅ Modüler paket yapısı
- ✅ Release builder (make-release.sh)
- ✅ Otomatik installer/uninstaller scriptleri
- ✅ Binary-only dağıtım (kaynak kod koruması)

### Güvenlik İyileştirmeleri
- ✅ TLS sertifika doğrulama (default: aktif)
- ✅ SSH known_hosts desteği
- ✅ Trust-on-first-use (TOFU) fallback
- ✅ Environment variable'dan password okuma
- ✅ SSH key passphrase desteği

### Breaking Changes
- Config dosyası artık opsiyonel (flag'ler tercih edilir)
- Eski `main.go` `main.go.backup` olarak saklandı

## İletişim

Sorular ve öneriler için: Netinternet Development Team
