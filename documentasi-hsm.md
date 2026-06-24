# Dokumentasi Generate SoftHSM Key, Public Key, CSR, Root CA, Sub CA, dan Signing Certificate

Dokumen ini menjelaskan cara membuat struktur certificate untuk PDF signing menggunakan SoftHSM.

Target akhirnya:

```text
Root CA
   |
   v
Sub CA
   |
   v
Signing Certificate
   |
   v
Private Key di SoftHSM
```

Contoh hasil di Adobe:

```text
Signed by: WSPDV TAMPAN
```

---

# 0. Konsep Singkat

SoftHSM menyimpan private key di token HSM lokal.

Yang boleh keluar:

```text
Public Key
CSR
Certificate
```

Yang tidak boleh keluar:

```text
Private Key pdf-sign-key
```

Untuk skenario shared key + multiple certificate:

```text
1 private key di HSM
1 public key yang sama
banyak CSR / certificate dengan CN berbeda
banyak Root CA / Sub CA berbeda
```

---

# 1. Install Package

```bash
sudo apt update
sudo apt install softhsm2 opensc libengine-pkcs11-openssl -y
```

Cek lokasi library SoftHSM:

```bash
find /usr -name libsofthsm2.so
```

Biasanya:

```text
/usr/lib/softhsm/libsofthsm2.so
```

atau:

```text
/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so
```

---

# 2. Setup Config SoftHSM

Jika ingin pakai config di home user:

```bash
mkdir -p ~/.softhsm/tokens
```

Buat config:

```bash
cat > ~/.softhsm/softhsm2.conf <<EOF
directories.tokendir = /home/$USER/.softhsm/tokens/
objectstore.backend = file
log.level = INFO
slots.removable = false
EOF
```

Export config:

```bash
export SOFTHSM2_CONF=/home/$USER/.softhsm/softhsm2.conf
```

Agar permanen:

```bash
echo 'export SOFTHSM2_CONF=/home/$USER/.softhsm/softhsm2.conf' >> ~/.bashrc
source ~/.bashrc
```

Cek slot:

```bash
softhsm2-util --show-slots
```

---

# 3. Init Token SoftHSM

```bash
softhsm2-util \
  --init-token \
  --slot 0 \
  --label "WSPDV-HSM"
```

Masukkan:

```text
SO PIN   : isi sendiri
User PIN : isi sendiri
```

Cek lagi:

```bash
softhsm2-util --show-slots
```

Pastikan muncul:

```text
Initialized: yes
User PIN init.: yes
Label: WSPDV-HSM
```

---

# 4. Generate Private Key di SoftHSM

Contoh generate RSA 3072 untuk signing PDF:

```bash
pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --login \
  --pin "ISI_USER_PIN_KAMU" \
  --keypairgen \
  --key-type rsa:3072 \
  --id 01 \
  --label pdf-sign-key
```

Cek key:

```bash
pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --login \
  --pin "ISI_USER_PIN_KAMU" \
  -O
```

Harus muncul:

```text
Private Key Object
  label: pdf-sign-key
  ID: 01
  Access: sensitive, always sensitive, never extractable

Public Key Object
  label: pdf-sign-key
  ID: 01
```

Catatan:

```text
Private key tidak keluar dari SoftHSM.
```

---

# 5. Export Public Key dari SoftHSM

Export public key DER:

```bash
pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --read-object \
  --type pubkey \
  --id 01 \
  > public.der
```

Convert DER ke PEM:

```bash
openssl rsa \
  -pubin \
  -inform DER \
  -in public.der \
  -outform PEM \
  -out public.pem
```

Cek public key:

```bash
cat public.pem
```

Output harus seperti:

```text
-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----
```

---

# 6. Generate Root CA

Root CA adalah certificate paling atas.

```bash
openssl genrsa -out root-ca.key 4096
```

```bash
openssl req \
  -x509 \
  -new \
  -nodes \
  -key root-ca.key \
  -sha256 \
  -days 3650 \
  -out root-ca.crt \
  -subj "/C=ID/O=WSPDV/CN=WSPDV Root CA"
```

Cek Root CA:

```bash
openssl x509 -in root-ca.crt -noout -subject -issuer
```

---

# 7. Generate Sub CA

Generate private key Sub CA:

```bash
openssl genrsa -out sub-ca.key 4096
```

Generate CSR Sub CA:

```bash
openssl req \
  -new \
  -key sub-ca.key \
  -out sub-ca.csr \
  -subj "/C=ID/O=WSPDV/CN=WSPDV PDF Sub CA"
```

Buat extension file:

```bash
cat > sub-ca.ext <<EOF
basicConstraints=critical,CA:TRUE,pathlen:0
keyUsage=critical,keyCertSign,cRLSign
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
EOF
```

Sign Sub CA menggunakan Root CA:

```bash
openssl x509 \
  -req \
  -in sub-ca.csr \
  -CA root-ca.crt \
  -CAkey root-ca.key \
  -CAcreateserial \
  -out sub-ca.crt \
  -days 1825 \
  -sha256 \
  -extfile sub-ca.ext
```

Cek Sub CA:

```bash
openssl x509 -in sub-ca.crt -noout -subject -issuer
```

Harus kira-kira:

```text
subject=C=ID, O=WSPDV, CN=WSPDV PDF Sub CA
issuer=C=ID, O=WSPDV, CN=WSPDV Root CA
```

---

# 8. Generate CSR Signing Certificate dari Key SoftHSM

CSR ini dibuat dari private key yang ada di SoftHSM.

Install engine jika belum:

```bash
sudo apt install libengine-pkcs11-openssl opensc -y
```

Cek engine:

```bash
openssl engine -t -c pkcs11
```

Generate CSR:

```bash
openssl req \
  -engine pkcs11 \
  -keyform engine \
  -key "pkcs11:token=WSPDV-HSM;object=pdf-sign-key;type=private" \
  -new \
  -out signing.csr \
  -subj "/C=ID/O=WSPDV/CN=WSPDV TAMPAN"
```

Catatan:

```text
CN=WSPDV TAMPAN
```

adalah nama yang biasanya muncul di Adobe sebagai:

```text
Signed by: WSPDV TAMPAN
```

---

# 9. Generate Signing Certificate dari Sub CA

Buat extension file untuk signing certificate:

```bash
cat > signing.ext <<EOF
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature
extendedKeyUsage=emailProtection,codeSigning
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
EOF
```

Sign CSR menggunakan Sub CA:

```bash
openssl x509 \
  -req \
  -in signing.csr \
  -CA sub-ca.crt \
  -CAkey sub-ca.key \
  -CAcreateserial \
  -out signing.crt \
  -days 825 \
  -sha256 \
  -extfile signing.ext
```

Cek signing certificate:

```bash
openssl x509 -in signing.crt -noout -subject -issuer
```

Harus kira-kira:

```text
subject=C=ID, O=WSPDV, CN=WSPDV TAMPAN
issuer=C=ID, O=WSPDV, CN=WSPDV PDF Sub CA
```

---

# 10. Buat Certificate Chain

Gabungkan Sub CA dan Root CA:

```bash
cat sub-ca.crt root-ca.crt > chain.crt
```

Atau fullchain:

```bash
cat signing.crt sub-ca.crt root-ca.crt > fullchain.crt
```

Verifikasi chain:

```bash
openssl verify \
  -CAfile chain.crt \
  signing.crt
```

Jika benar:

```text
signing.crt: OK
```

---

# 11. Verifikasi Public Key Signing Certificate Sama dengan SoftHSM

Export public key dari signing certificate:

```bash
openssl x509 -in signing.crt -pubkey -noout > signing-public.pem
```

Bandingkan dengan public key dari HSM:

```bash
diff public.pem signing-public.pem
```

Jika tidak ada output, berarti sama.

---

# 12. File yang Dihasilkan

## Private Key

```text
root-ca.key
sub-ca.key
```

Ini rahasia.

Catatan:

```text
Private key pdf-sign-key tidak berbentuk file.
Dia ada di SoftHSM.
```

## Public Key

```text
public.der
public.pem
```

## CSR

```text
sub-ca.csr
signing.csr
```

## Certificate

```text
root-ca.crt
sub-ca.crt
signing.crt
```

## Chain

```text
chain.crt
fullchain.crt
```

## Extension Config

```text
sub-ca.ext
signing.ext
```

## Serial Number

```text
root-ca.srl
sub-ca.srl
```

---

# 13. Untuk HexaPDF / PDF Signing

Biasanya yang dibutuhkan:

```text
signing.crt
chain.crt
```

Private key tidak dibaca dari file.

Signature dibuat oleh:

```text
SoftHSM
  token: WSPDV-HSM
  key: pdf-sign-key
```

Flow:

```text
HexaPDF / Rails
    |
    v
Hash PDF
    |
    v
Go hash-signing-service
    |
    v
SoftHSM sign pakai pdf-sign-key
    |
    v
Signature bytes
    |
    v
Embed ke PDF bersama signing.crt + chain.crt
```

---

# 14. Membuat Signed By Berbeda dengan Key yang Sama

Jika ingin Adobe menampilkan nama berbeda:

```text
Signed by: Partner A
Signed by: Partner B
Signed by: WSPDV TAMPAN
```

buat CSR berbeda dari private key SoftHSM yang sama.

Contoh Partner A:

```bash
openssl req \
  -engine pkcs11 \
  -keyform engine \
  -key "pkcs11:token=WSPDV-HSM;object=pdf-sign-key;type=private" \
  -new \
  -out signing-partner-a.csr \
  -subj "/C=ID/O=Partner A/CN=Partner A"
```

Issue certificate dari Sub CA:

```bash
openssl x509 \
  -req \
  -in signing-partner-a.csr \
  -CA sub-ca.crt \
  -CAkey sub-ca.key \
  -CAcreateserial \
  -out signing-partner-a.crt \
  -days 825 \
  -sha256 \
  -extfile signing.ext
```

Catatan penting:

```text
Private key sama.
Public key sama.
Certificate berbeda.
CN berbeda.
Adobe bisa menampilkan Signed by berbeda.
```

Namun secara kriptografi:

```text
Identitas key tetap sama.
```

Untuk identity isolation yang benar-benar berbeda, gunakan private key berbeda per partner.

---

# 15. Membuat Root CA / Sub CA Berbeda untuk Partner Berbeda

## Tujuan

Menggunakan **private key yang sama di SoftHSM**, tetapi memiliki certificate dan certificate chain yang berbeda untuk setiap partner.

Arsitektur:

```text
SoftHSM
└── pdf-sign-key
        |
        +--> signing.crt
        |
        +--> signing-ciba.crt
        |
        +--> signing-partner-a.crt
        |
        +--> signing-partner-b.crt
```

Semua certificate di atas menggunakan:

```text
Private Key = sama
Public Key  = sama
```

Namun Adobe dapat menampilkan identitas yang berbeda:

```text
Signed by: WSPDV TAMPAN
Signed by: CIBA TAMPAN
Signed by: Partner A
Signed by: Partner B
```

Karena certificate dan certificate chain yang digunakan berbeda.

---

## Step 1 - Generate Root CA Partner

Contoh untuk partner CIBA.

Generate private key Root CA:

```bash
openssl genrsa -out ciba-root-ca.key 4096
```

Generate Root CA Certificate:

```bash
openssl req \
  -x509 \
  -new \
  -nodes \
  -key ciba-root-ca.key \
  -sha256 \
  -days 3650 \
  -out ciba-root.crt \
  -subj "/C=ID/O=CIBA/CN=CIBA Root CA"
```

Verifikasi:

```bash
openssl x509 -in ciba-root.crt -noout -subject -issuer
```

Output kira-kira:

```text
subject=C=ID, O=CIBA, CN=CIBA Root CA
issuer=C=ID, O=CIBA, CN=CIBA Root CA
```

---

## Step 2 - Generate Sub CA Partner

Generate private key Sub CA:

```bash
openssl genrsa -out ciba-sub.key 4096
```

Generate CSR Sub CA:

```bash
openssl req \
  -new \
  -key ciba-sub.key \
  -out ciba-sub.csr \
  -subj "/C=ID/O=CIBA/CN=CIBA Sub CA"
```

Issue Sub CA menggunakan Root CA:

```bash
openssl x509 \
  -req \
  -in ciba-sub.csr \
  -CA ciba-root.crt \
  -CAkey ciba-root-ca.key \
  -CAcreateserial \
  -out ciba-sub.crt \
  -days 1825 \
  -sha256 \
  -extfile sub-ca.ext
```

Verifikasi:

```bash
openssl x509 -in ciba-sub.crt -noout -subject -issuer
```

Output kira-kira:

```text
subject=C=ID, O=CIBA, CN=CIBA Sub CA
issuer=C=ID, O=CIBA, CN=CIBA Root CA
```

---

## Step 3 - Generate CSR dari SoftHSM

CSR dibuat menggunakan private key yang sama di SoftHSM.

Generate CSR untuk CIBA:

```bash
openssl req \
  -engine pkcs11 \
  -keyform engine \
  -key "pkcs11:token=WSPDV-HSM;object=pdf-sign-key;type=private" \
  -new \
  -out signing-ciba.csr \
  -subj "/C=ID/O=CIBA/CN=CIBA TAMPAN"
```

Catatan:

```text
CN=CIBA TAMPAN
```

adalah nama yang biasanya akan muncul di Adobe:

```text
Signed by: CIBA TAMPAN
```

---

## Step 4 - Generate Signing Certificate Partner

Issue certificate menggunakan Sub CA partner.

Generate Signing Certificate:

```bash
openssl x509 \
  -req \
  -in signing-ciba.csr \
  -CA ciba-sub.crt \
  -CAkey ciba-sub.key \
  -CAcreateserial \
  -out signing-ciba.crt \
  -days 825 \
  -sha256 \
  -extfile signing.ext
```

Verifikasi:

```bash
openssl x509 -in signing-ciba.crt -noout -subject -issuer
```

Output kira-kira:

```text
subject=C=ID, O=CIBA, CN=CIBA TAMPAN
issuer=C=ID, O=CIBA, CN=CIBA Sub CA
```

---

## Step 5 - Generate Certificate Chain Partner

Buat certificate chain yang nantinya akan di-embed ke PDF.

Generate chain:

```bash
cat ciba-sub.crt ciba-root.crt > ciba-chain.crt
```

Optional fullchain:

```bash
cat signing-ciba.crt ciba-sub.crt ciba-root.crt > ciba-fullchain.crt
```

---

## Step 6 - Verifikasi Chain

Verifikasi bahwa chain valid:

```bash
openssl verify \
  -CAfile ciba-chain.crt \
  signing-ciba.crt
```

Output:

```text
signing-ciba.crt: OK
```

Jika output bukan OK, berarti ada masalah pada chain.

---

## File yang Digunakan Saat PDF Signing

### Private Key

Disimpan di SoftHSM:

```text
Token : WSPDV-HSM
Key   : pdf-sign-key
```

### Certificate

```text
signing-ciba.crt
```

### Certificate Chain

```text
ciba-chain.crt
```

atau:

```text
ciba-sub.crt
ciba-root.crt
```

---

## Hasil di Adobe

Adobe akan menampilkan:

```text
Signed by: CIBA TAMPAN
Issued by: CIBA Sub CA
```

Walaupun:

```text
Private Key = sama
Public Key  = sama
```

dengan certificate lain seperti:

```text
signing.crt
signing-partner-a.crt
signing-partner-b.crt
```

Ini adalah implementasi **Shared Private Key + Multiple Certificates (Opsi 1)**.

---

## Ringkasan Alur

```text
SoftHSM
└── pdf-sign-key
      |
      v
signing-ciba.csr
      |
      v
signing-ciba.crt
      |
      v
ciba-chain.crt
      |
      v
HexaPDF / PDF Signing
      |
      v
Adobe Reader
      |
      v
Signed by: CIBA TAMPAN
```

---

# 16. Catatan Adobe Trust

Jika Root CA self-signed:

```text
Adobe bisa menampilkan signature valid secara kriptografi,
tapi root CA belum trusted.
```

Biasanya muncul:

```text
Signer identity is unknown
```

Agar trusted:

1. Import Root CA ke Adobe Trusted Certificates, atau
2. Deploy Root CA via MDM/GPO untuk internal enterprise, atau
3. Gunakan CA yang masuk AATL seperti GlobalSign.

Untuk PoC dan development, self-signed Root CA sudah cukup.

---

# 17. Import Private Key ke AWS CloudHSM

Bagian ini untuk memindahkan private key existing ke AWS CloudHSM supaya service
`hash-signing-service` tetap memakai backend PKCS#11, tetapi module-nya bukan
SoftHSM.

Target akhirnya:

```text
AWS CloudHSM Cluster
└── Crypto User (CU)
    └── Private Key Object
        label: msign-key
        id   : 01
        usage: sign
```

Service akan sign lewat:

```dotenv
SIGNER_BACKEND=pkcs11
HSM_MODULE_PATH=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
HSM_TOKEN_LABEL=<TOKEN_LABEL_DARI_PKCS11_LIST_SLOTS>
HSM_PIN=<CU_USER>:<CU_PASSWORD>
HSM_KEY_LABEL=msign-key
HSM_KEY_ID=01
```

Catatan penting:

```text
Untuk AWS CloudHSM, HSM_PIN bukan hanya password.
Format PIN PKCS#11 adalah:

<CU_USER>:<CU_PASSWORD>
```

Contoh:

```dotenv
HSM_PIN=app_pdf_signer:SuperSecretPassword123!
```

Referensi AWS: PKCS#11 Client SDK 5 memakai format PIN
`<CU_user_name>:<password>`.

---

## 17.1 Prerequisite AWS CloudHSM

Pastikan sudah ada:

```text
1. AWS CloudHSM cluster aktif
2. Minimal 1 HSM aktif di cluster
3. EC2/app host berada di VPC/subnet/security group yang bisa akses HSM
4. AWS CloudHSM Client SDK 5 terinstall di EC2/app host
5. Cluster sudah di-bootstrap/configure
6. Crypto User (CU) untuk aplikasi sudah dibuat
```

Install PKCS#11 Client SDK 5 sesuai OS.

Contoh Ubuntu 22.04:

```bash
wget https://s3.amazonaws.com/cloudhsmv2-software/CloudHsmClient/Jammy/cloudhsm-pkcs11_latest_u22.04_amd64.deb
sudo apt install ./cloudhsm-pkcs11_latest_u22.04_amd64.deb
```

Contoh Ubuntu 24.04:

```bash
wget https://s3.amazonaws.com/cloudhsmv2-software/CloudHsmClient/Noble/cloudhsm-pkcs11_latest_u24.04_amd64.deb
sudo apt install ./cloudhsm-pkcs11_latest_u24.04_amd64.deb
```

Lokasi umum file AWS CloudHSM di Linux:

```text
/opt/cloudhsm
```

Module PKCS#11 yang dipakai service:

```text
/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
```

Jika path berbeda, cari:

```bash
sudo find /opt/cloudhsm -name 'libcloudhsm_pkcs11.so'
```

---

## 17.2 Buat Crypto User (CU)

Jika belum ada user untuk aplikasi, buat CU dari user CO/PRECO.

Masuk ke CloudHSM Management Utility atau CloudHSM CLI sesuai setup cluster.
Contoh konsep dengan `cloudhsm_mgmt_util`:

```text
aws-cloudhsm> loginHSM CO admin <CO_PASSWORD>
aws-cloudhsm> createUser CU app_pdf_signer <CU_PASSWORD>
aws-cloudhsm> listUsers
```

CU inilah yang dipakai aplikasi untuk login ke PKCS#11.

Untuk service ini:

```dotenv
HSM_PIN=app_pdf_signer:<CU_PASSWORD>
```

Jangan pakai CO user untuk aplikasi runtime.

---

## 17.3 Siapkan File Key dan Certificate

Contoh folder:

```text
certs/msign/private.pem
certs/msign/signing.pem
certs/msign/signing.crt
certs/msign/sub-ca.crt
certs/msign/root-ca.crt
```

Validasi private key:

```bash
openssl rsa -in certs/msign/private.pem -check -noout
```

Output harus:

```text
RSA key ok
```

Bandingkan modulus private key dengan certificate:

```bash
openssl pkey \
  -in certs/msign/private.pem \
  -pubout \
  -outform PEM \
| openssl rsa -pubin -noout -modulus

openssl x509 \
  -in certs/msign/signing.crt \
  -noout \
  -modulus
```

Kedua modulus harus sama.

Jika modulus berbeda:

```text
Jangan import ke HSM.
Certificate tidak pair dengan private key.
Signature PDF akan invalid.
```

---

## 17.4 Convert Private Key ke PKCS#8 DER

`pkcs11-tool --write-object` lebih aman memakai input DER.

Convert private key ke PKCS#8 DER:

```bash
openssl pkcs8 \
  -topk8 \
  -nocrypt \
  -in certs/msign/private.pem \
  -outform DER \
  -out /tmp/msign-private.pk8.der
```

Convert public key PEM ke DER:

```bash
openssl pkey \
  -pubin \
  -in certs/msign/signing.pem \
  -outform DER \
  -out /tmp/msign-public.der
```

Jika tidak punya `signing.pem`, extract dari `signing.crt`:

```bash
openssl x509 \
  -in certs/msign/signing.crt \
  -pubkey \
  -noout \
  > /tmp/msign-signing-public.pem

openssl pkey \
  -pubin \
  -in /tmp/msign-signing-public.pem \
  -outform DER \
  -out /tmp/msign-public.der
```

---

## 17.5 Cek Token Label AWS CloudHSM

List slot/token dari module AWS CloudHSM:

```bash
pkcs11-tool \
  --module /opt/cloudhsm/lib/libcloudhsm_pkcs11.so \
  --list-slots
```

Catat nilai:

```text
token label : <TOKEN_LABEL>
```

Nilai ini harus dipakai untuk:

```dotenv
HSM_TOKEN_LABEL=<TOKEN_LABEL>
```

Jangan menebak label token. Pakai persis dari output `--list-slots`, karena
service mencari slot berdasarkan token label.

---

## 17.6 Import Private Key ke AWS CloudHSM

Set variable agar command tidak terlalu panjang:

```bash
export AWS_CLOUDHSM_PKCS11=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
export HSM_TOKEN_LABEL="<TOKEN_LABEL_DARI_LIST_SLOTS>"
export HSM_CU_USER="app_pdf_signer"
export HSM_CU_PASSWORD="<CU_PASSWORD>"
export HSM_PIN="${HSM_CU_USER}:${HSM_CU_PASSWORD}"
export HSM_KEY_LABEL="msign-key"
export HSM_KEY_ID="01"
```

Import private key:

```bash
pkcs11-tool \
  --module "$AWS_CLOUDHSM_PKCS11" \
  --login \
  --pin "$HSM_PIN" \
  --token-label "$HSM_TOKEN_LABEL" \
  --write-object /tmp/msign-private.pk8.der \
  --type privkey \
  --id "$HSM_KEY_ID" \
  --label "$HSM_KEY_LABEL" \
  --usage-sign \
  --private \
  --sensitive
```

Import public key object dengan id/label yang sama:

```bash
pkcs11-tool \
  --module "$AWS_CLOUDHSM_PKCS11" \
  --login \
  --pin "$HSM_PIN" \
  --token-label "$HSM_TOKEN_LABEL" \
  --write-object /tmp/msign-public.der \
  --type pubkey \
  --id "$HSM_KEY_ID" \
  --label "$HSM_KEY_LABEL" \
  --usage-sign
```

Catatan:

```text
Service ini hanya butuh private key object untuk signing.
Public key object tetap disarankan agar pengecekan object di HSM lebih mudah.
```

Jika AWS CloudHSM policy menolak direct import private key dengan
`pkcs11-tool --write-object`, gunakan flow import resmi CloudHSM berbasis key
wrapping/unwrapping dari CloudHSM CLI atau key_mgmt_util. Prinsipnya tetap sama:

```text
1. Buat wrapping key di HSM.
2. Wrap private key di luar HSM sesuai mekanisme yang diizinkan.
3. Unwrap/import private key ke HSM sebagai key non-extractable.
4. Set label = msign-key, id = 01, usage = sign.
```

Setelah import, command verifikasi di section berikut tetap sama.

---

## 17.7 Cek Object di AWS CloudHSM

List object:

```bash
pkcs11-tool \
  --module "$AWS_CLOUDHSM_PKCS11" \
  --login \
  --pin "$HSM_PIN" \
  --token-label "$HSM_TOKEN_LABEL" \
  --list-objects
```

Minimal harus terlihat:

```text
Private Key Object; RSA
  label:      msign-key
  ID:         01
  Usage:      sign
```

Jika ada public key object:

```text
Public Key Object; RSA 2048 bits
  label:      msign-key
  ID:         01
  Usage:      verify
```

Export/check public key object:

```bash
pkcs11-tool \
  --module "$AWS_CLOUDHSM_PKCS11" \
  --login \
  --pin "$HSM_PIN" \
  --token-label "$HSM_TOKEN_LABEL" \
  --read-object \
  --type pubkey \
  --id "$HSM_KEY_ID" \
| openssl rsa -pubin -inform DER -noout -modulus
```

Bandingkan dengan:

```bash
openssl x509 -in certs/msign/signing.crt -noout -modulus
```

Modulus harus sama.

---

## 17.8 Verifikasi Signing dari AWS CloudHSM

Buat sample message:

```bash
openssl rand -out /tmp/msign-message.bin 64
openssl x509 -in certs/msign/signing.crt -pubkey -noout -out /tmp/msign-cert-public.pem
```

Sign memakai AWS CloudHSM.

Untuk test dengan `pkcs11-tool`:

```bash
pkcs11-tool \
  --module "$AWS_CLOUDHSM_PKCS11" \
  --login \
  --pin "$HSM_PIN" \
  --token-label "$HSM_TOKEN_LABEL" \
  --sign \
  --mechanism SHA256-RSA-PKCS \
  --id "$HSM_KEY_ID" \
  --input-file /tmp/msign-message.bin \
  --output-file /tmp/msign-signature.bin
```

Verify:

```bash
openssl dgst \
  -sha256 \
  -verify /tmp/msign-cert-public.pem \
  -signature /tmp/msign-signature.bin \
  /tmp/msign-message.bin
```

Output harus:

```text
Verified OK
```

Catatan untuk service:

```text
hash-signing-service tidak memakai SHA256-RSA-PKCS.
Service memakai CKM_RSA_PKCS dan menambahkan DigestInfo sendiri,
karena input dari msign-backend adalah pre-computed hash.
```

Jadi test `pkcs11-tool` di atas hanya untuk membuktikan key pair benar dan HSM
bisa signing. Path runtime service tetap divalidasi dengan endpoint `/api/v1/hash-sign`
atau test Go yang memanggil `HSMSigner`.

---

## 17.9 Env hash-signing-service untuk AWS CloudHSM

Contoh `.env`:

```dotenv
SIGNER_BACKEND=pkcs11

CERT_FILE=certs/msign/signing.crt
CERT_KEY_FILE=certs/msign/private.pem
CERT_SUB_CA_FILE=certs/msign/sub-ca.crt
CERT_ROOT_CA_FILE=certs/msign/root-ca.crt

HSM_MODULE_PATH=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
HSM_TOKEN_LABEL=<TOKEN_LABEL_DARI_LIST_SLOTS>
HSM_PIN=app_pdf_signer:<CU_PASSWORD>
HSM_KEY_LABEL=msign-key
HSM_KEY_ID=01
```

Untuk production:

```text
Jangan simpan HSM_PIN plaintext di repo.
Inject dari secret manager, environment Kubernetes, ECS secret, atau mekanisme
secret runtime lain.
```

Restart service setelah env berubah.

---

## 17.10 Troubleshooting AWS CloudHSM

### `token with label "... " not found`

Penyebab:

```text
HSM_TOKEN_LABEL tidak sama dengan output pkcs11-tool --list-slots.
```

Fix:

```bash
pkcs11-tool --module /opt/cloudhsm/lib/libcloudhsm_pkcs11.so --list-slots
```

Copy token label persis ke `.env`.

### `login failed`

Penyebab umum:

```text
HSM_PIN salah format.
AWS CloudHSM butuh <CU_USER>:<CU_PASSWORD>.
```

Contoh benar:

```dotenv
HSM_PIN=app_pdf_signer:SuperSecretPassword123!
```

### `private key with label "msign-key" (id: "01") not found`

Cek object:

```bash
pkcs11-tool \
  --module /opt/cloudhsm/lib/libcloudhsm_pkcs11.so \
  --login \
  --pin "app_pdf_signer:<CU_PASSWORD>" \
  --token-label "<TOKEN_LABEL>" \
  --list-objects
```

Pastikan private key punya:

```text
label = msign-key
ID    = 01
```

### Signature PDF invalid di Adobe

Cek berurutan:

```text
1. Private key HSM pair dengan signing.crt.
2. msign-backend embed signing.crt yang benar.
3. Chain sub-ca/root-ca benar.
4. Certificate belum expired.
5. Input ke hash-signing-service adalah digest, bukan raw PDF bytes.
6. hash_algo OID sesuai panjang digest.
```

OID yang didukung service:

```text
SHA-256: 2.16.840.1.101.3.4.2.1 -> 32 byte digest
SHA-384: 2.16.840.1.101.3.4.2.2 -> 48 byte digest
SHA-512: 2.16.840.1.101.3.4.2.3 -> 64 byte digest
```

---

## 17.11 Referensi AWS

```text
AWS CloudHSM Client SDK 5 PKCS#11 library:
https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-library.html

Install PKCS#11 library:
https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-library-install.html

Authenticate to PKCS#11 library:
https://docs.aws.amazon.com/cloudhsm/latest/userguide/pkcs11-pin.html

Create AWS CloudHSM user with CMU:
https://docs.aws.amazon.com/cloudhsm/latest/userguide/cloudhsm_mgmt_util-createUser.html
```

---

# 18. Contoh Praktis: Import Private Key dari File PEM ke Token HSM Baru

Bagian ini adalah contoh step-by-step seperti proses refresh MSIGN yang dilakukan
di local/dev HSM.

Kondisi awal:

```text
Kita mendapatkan private key dari provider/HSM source dalam bentuk file PEM.
Kita juga mendapatkan public key PEM.
Certificate signing.crt sudah tersedia di certs/msign.
Token HSM lama ingin dihapus dan diganti token baru.
```

Target akhirnya:

```text
certs/msign/private.pem  -> private key valid
certs/msign/signing.pem  -> raw public key PEM
certs/msign/signing.crt  -> certificate untuk CMS/PDF

SoftHSM/AWS CloudHSM token:
  token label : MSIGN-HSM
  key label   : msign-key
  key id      : 01
  usage       : sign

.env:
  SIGNER_BACKEND=pkcs11
  HSM_TOKEN_LABEL=MSIGN-HSM
  HSM_KEY_LABEL=msign-key
  HSM_KEY_ID=01
```

Catatan:

```text
Untuk PDF signing, msign-backend tetap memakai signing.crt.
signing.pem hanya raw public key untuk debug/verifikasi manual.
```

---

## 18.1 Simpan Private Key PEM

Replace isi file:

```text
certs/msign/private.pem
```

Format yang benar:

```text
-----BEGIN PRIVATE KEY-----
...
-----END PRIVATE KEY-----
```

atau:

```text
-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----
```

Setelah file diganti, validasi:

```bash
openssl rsa -in certs/msign/private.pem -check -noout
```

Output harus:

```text
RSA key ok
```

Jika muncul error seperti ini:

```text
p not prime
q not prime
n does not equal p q
d e not congruent to 1
```

berarti private key rusak/dummy. Jangan import ke HSM, karena signature yang
dihasilkan akan invalid walaupun modulus terlihat sama.

---

## 18.2 Simpan Public Key PEM sebagai signing.pem

Simpan raw public key ke:

```text
certs/msign/signing.pem
```

Format:

```text
-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----
```

File ini bukan certificate. Certificate tetap:

```text
certs/msign/signing.crt
```

---

## 18.3 Pastikan Private Key, Public Key, dan Certificate Pair

Cek modulus private key:

```bash
openssl pkey \
  -in certs/msign/private.pem \
  -pubout \
  -outform PEM \
| openssl rsa -pubin -noout -modulus
```

Cek modulus public key:

```bash
openssl rsa \
  -pubin \
  -in certs/msign/signing.pem \
  -noout \
  -modulus
```

Cek modulus certificate:

```bash
openssl x509 \
  -in certs/msign/signing.crt \
  -noout \
  -modulus
```

Ketiganya harus sama.

Cek masa berlaku certificate:

```bash
openssl x509 \
  -in certs/msign/signing.crt \
  -noout \
  -subject \
  -serial \
  -dates
```

Jika certificate expired, PDF signature baru akan invalid di Adobe walaupun
private key dan public key pair.

---

## 18.4 Hapus Token HSM Lama

Untuk SoftHSM local/dev:

```bash
softhsm2-util --show-slots
```

Jika token lama ada:

```text
Label: MSIGN-HSM
```

hapus token lama:

```bash
softhsm2-util --delete-token --token MSIGN-HSM
```

Pastikan token hilang:

```bash
softhsm2-util --show-slots
```

Catatan AWS CloudHSM:

```text
Di AWS CloudHSM biasanya tidak ada konsep delete token seperti SoftHSM.
Yang dihapus adalah key object di dalam cluster, bukan token/slot.
Gunakan tool AWS CloudHSM yang sesuai policy operasional.
```

---

## 18.5 Buat Token HSM Baru

Untuk SoftHSM local/dev:

```bash
softhsm2-util \
  --init-token \
  --free \
  --label MSIGN-HSM \
  --pin anaktampan \
  --so-pin anaktampan-so
```

Cek token baru:

```bash
softhsm2-util --show-slots
```

Harus terlihat:

```text
Initialized: yes
User PIN init.: yes
Label: MSIGN-HSM
```

Untuk AWS CloudHSM:

```text
Token/slot berasal dari cluster AWS CloudHSM.
Jangan init token manual seperti SoftHSM.
Gunakan token label yang muncul dari pkcs11-tool --list-slots.
```

---

## 18.6 Import Private Key ke Token Baru

Untuk SoftHSM, private key **PEM PKCS#8** bisa langsung di-import dengan
`softhsm2-util`.

Pastikan header file:

```text
-----BEGIN PRIVATE KEY-----
```

Jika private key masih format PKCS#1:

```text
-----BEGIN RSA PRIVATE KEY-----
```

convert dulu ke PKCS#8 PEM:

```bash
openssl pkcs8 \
  -topk8 \
  -nocrypt \
  -in certs/msign/private.pem \
  -out /tmp/msign-private-pkcs8.pem
```

Lalu import file hasil convert:

```bash
softhsm2-util \
  --import /tmp/msign-private-pkcs8.pem \
  --token MSIGN-HSM \
  --pin anaktampan \
  --label msign-key \
  --id 01
```

Jika private key sudah PKCS#8 PEM, seperti proses MSIGN yang dilakukan, import
langsung:

```bash
softhsm2-util \
  --import certs/msign/private.pem \
  --token MSIGN-HSM \
  --pin anaktampan \
  --label msign-key \
  --id 01
```

Untuk AWS CloudHSM atau import via `pkcs11-tool --write-object`, gunakan
PKCS#8 DER seperti section 17.4:

```bash
openssl pkcs8 \
  -topk8 \
  -nocrypt \
  -in certs/msign/private.pem \
  -outform DER \
  -out /tmp/msign-private.pk8.der
```

Output sukses:

```text
The key pair has been imported.
```

Cek object:

```bash
pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --login \
  --pin anaktampan \
  --token-label MSIGN-HSM \
  --list-objects
```

Harus ada:

```text
Private Key Object; RSA
  label:      msign-key
  ID:         01
  Usage:      sign

Public Key Object; RSA 2048 bits
  label:      msign-key
  ID:         01
```

Jika `pkcs11-tool --list-objects` menampilkan object lalu keluar
`CKR_GENERAL_ERROR`, cek lagi dengan runtime aplikasi. Di local dev, kasus itu
bisa terjadi pada tool listing, sementara object tetap bisa dipakai signing.

---

## 18.7 Verifikasi Public Key Object HSM Match dengan signing.crt

Export public key object dari HSM dan cek modulus:

```bash
pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --login \
  --pin anaktampan \
  --token-label MSIGN-HSM \
  --read-object \
  --type pubkey \
  --id 01 \
| openssl rsa -pubin -inform DER -noout -modulus
```

Bandingkan dengan:

```bash
openssl x509 -in certs/msign/signing.crt -noout -modulus
```

Modulus harus sama.

---

## 18.8 Update .env

Untuk local/dev SoftHSM:

```dotenv
SIGNER_BACKEND=pkcs11

CERT_FILE=certs/msign/signing.crt
CERT_KEY_FILE=certs/msign/private.pem
CERT_SUB_CA_FILE=certs/msign/sub-ca.crt
CERT_ROOT_CA_FILE=certs/msign/root-ca.crt

HSM_MODULE_PATH=/usr/lib/softhsm/libsofthsm2.so
HSM_TOKEN_LABEL=MSIGN-HSM
HSM_PIN=anaktampan
HSM_KEY_LABEL=msign-key
HSM_KEY_ID=01
```

Untuk AWS CloudHSM:

```dotenv
SIGNER_BACKEND=pkcs11

CERT_FILE=certs/msign/signing.crt
CERT_KEY_FILE=certs/msign/private.pem
CERT_SUB_CA_FILE=certs/msign/sub-ca.crt
CERT_ROOT_CA_FILE=certs/msign/root-ca.crt

HSM_MODULE_PATH=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
HSM_TOKEN_LABEL=<TOKEN_LABEL_DARI_PKCS11_LIST_SLOTS>
HSM_PIN=<CU_USER>:<CU_PASSWORD>
HSM_KEY_LABEL=msign-key
HSM_KEY_ID=01
```

Perbedaan penting:

```text
SoftHSM:
  HSM_PIN=anaktampan

AWS CloudHSM:
  HSM_PIN=<CU_USER>:<CU_PASSWORD>
```

---

## 18.9 Verifikasi Runtime Service

Minimal cek unit test:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Untuk verifikasi runtime HSM, buat test kecil atau jalankan endpoint service
yang melakukan:

```text
1. NewHSMSigner(module, token label, pin, key label, key id)
2. Sign digest SHA-256
3. Verify signature dengan public key dari certs/msign/signing.crt
```

Expected result:

```text
HSM signer init sukses.
Signature dari HSM valid terhadap signing.crt.
```

Jika hasil verify gagal:

```text
1. Cek private key valid dengan openssl rsa -check.
2. Cek modulus private/public/cert.
3. Cek object HSM yang dipakai benar label/id-nya.
4. Cek msign-backend embed signing.crt yang benar.
```

---

## 18.10 Checklist Sukses

Sebelum dipakai oleh `msign-backend`, pastikan:

```text
[x] certs/msign/private.pem RSA key ok
[x] certs/msign/signing.pem match dengan private.pem
[x] certs/msign/signing.crt match dengan private.pem
[x] signing.crt belum expired
[x] token HSM MSIGN-HSM ada
[x] private key object label msign-key id 01 ada
[x] HSM_KEY_LABEL=msign-key
[x] HSM_KEY_ID=01
[x] HSM_TOKEN_LABEL=MSIGN-HSM
[x] signature HSM verified OK against signing.crt
```

---

# 19. AWS KMS sebagai Alternatif CloudHSM Dedicated

Bagian ini menjelaskan cara memakai AWS KMS sebagai backend signing untuk
memotong biaya dari AWS CloudHSM Dedicated (~$5.000/bulan) menjadi ~$1/bulan.

## 19.1 Perbandingan

```text
Backend            Biaya/bulan   FIPS Level   Non-exportable   Setup
------------------------------------------------------------------
CloudHSM Dedicated ~$5.000       Level 3      Ya               Cluster + EC2
AWS KMS            ~$1           Level 3      Ya (setelah import) API/SDK
SoftHSM            Gratis        Tidak ada    Tidak            Dev only
```

AWS KMS memakai shared HSM (multi-tenant), tetapi setiap key terisolasi.
Key tidak pernah keluar dari HSM AWS.

## 19.2 Dua Pilihan Key di KMS

### Pilihan A: Generate Key Baru di KMS

Key dibuat langsung di KMS. Private key tidak pernah ada di luar HSM.
Ini opsi paling aman secara compliance.

Konsekuensi: perlu request certificate baru dari GlobalSign dengan public key baru.

### Pilihan B: Import Private Key Existing ke KMS

Private key yang sudah ada (misalnya dari CloudHSM) di-import ke KMS.
Setelah import, key non-exportable dari KMS.

Konsekuensi: bisa pakai certificate yang sudah ada tanpa request ulang ke GlobalSign.
Catatan compliance: private key sempat ada dalam bentuk file sebelum import.

---

## 19.3 Pilihan A: Buat Key Baru di KMS

```bash
aws kms create-key \
  --key-spec RSA_2048 \
  --key-usage SIGN_VERIFY \
  --description "msign-aatl-signing-key" \
  --region ap-southeast-1
```

Catat `KeyId` dari output. Beri alias agar mudah direferensi:

```bash
aws kms create-alias \
  --alias-name alias/msign-aatl \
  --target-key-id <KeyId> \
  --region ap-southeast-1
```

Export public key untuk request CSR ke GlobalSign:

```bash
aws kms get-public-key \
  --key-id alias/msign-aatl \
  --region ap-southeast-1 \
  --output text \
  --query PublicKey \
  | base64 --decode > msign-kms-public.der

openssl pkey \
  -pubin \
  -inform DER \
  -in msign-kms-public.der \
  -outform PEM \
  -out msign-kms-public.pem
```

Kirim `msign-kms-public.pem` ke GlobalSign untuk di-sign menjadi AATL certificate.

---

## 19.4 Pilihan B: Import Private Key Existing ke KMS

### Langkah 1 — Buat KMS Key dengan EXTERNAL origin

```bash
aws kms create-key \
  --key-spec RSA_2048 \
  --key-usage SIGN_VERIFY \
  --origin EXTERNAL \
  --description "msign-aatl-imported" \
  --region ap-southeast-1
```

Catat `KeyId`.

### Langkah 2 — Download wrapping key dan import token

```bash
aws kms get-parameters-for-import \
  --key-id <KeyId> \
  --wrapping-algorithm RSAES_OAEP_SHA_256 \
  --wrapping-key-spec RSA_2048 \
  --region ap-southeast-1 \
  --output json > import-params.json

# Extract wrapping public key
cat import-params.json | python3 -c \
  "import sys,json,base64; d=json.load(sys.stdin); open('wrapping-key.der','wb').write(base64.b64decode(d['PublicKey']))"

# Extract import token
cat import-params.json | python3 -c \
  "import sys,json,base64; d=json.load(sys.stdin); open('import-token.bin','wb').write(base64.b64decode(d['ImportToken']))"
```

### Langkah 3 — Encrypt private key dengan wrapping key

```bash
# Convert private key ke PKCS#8 DER
openssl pkcs8 \
  -topk8 \
  -nocrypt \
  -in certs/msign/private.pem \
  -outform DER \
  -out /tmp/msign-private.pk8.der

# Encrypt dengan wrapping key KMS
openssl pkeyutl \
  -encrypt \
  -pubin \
  -inkey wrapping-key.der \
  -keyform DER \
  -pkeyopt rsa_padding_mode:oaep \
  -pkeyopt rsa_oaep_md:sha256 \
  -in /tmp/msign-private.pk8.der \
  -out /tmp/msign-encrypted-key.bin
```

### Langkah 4 — Import ke KMS

```bash
aws kms import-key-material \
  --key-id <KeyId> \
  --encrypted-key-material fileb:///tmp/msign-encrypted-key.bin \
  --import-token fileb://import-token.bin \
  --expiration-model KEY_MATERIAL_DOES_NOT_EXPIRE \
  --region ap-southeast-1
```

Output sukses:

```text
(tidak ada error)
```

Setelah import, private key tidak bisa di-export dari KMS.

### Langkah 5 — Verifikasi key aktif

```bash
aws kms describe-key \
  --key-id <KeyId> \
  --region ap-southeast-1 \
  --query 'KeyMetadata.{Status:KeyState,Origin:Origin,Usage:KeyUsage}'
```

Output harus:

```text
{
  "Status": "Enabled",
  "Origin": "EXTERNAL",
  "Usage": "SIGN_VERIFY"
}
```

### Langkah 6 — Test signing

```bash
# Buat digest SHA-256 dari random data
openssl rand 32 > /tmp/test-digest.bin

# Sign dengan KMS
aws kms sign \
  --key-id <KeyId> \
  --message fileb:///tmp/test-digest.bin \
  --message-type DIGEST \
  --signing-algorithm RSASSA_PKCS1_V1_5_SHA_256 \
  --region ap-southeast-1 \
  --output text \
  --query Signature \
  | base64 --decode > /tmp/test-sig.bin

# Verify dengan public key dari signing.crt
openssl x509 -in certs/msign/signing.crt -pubkey -noout > /tmp/kms-pubkey.pem
openssl dgst \
  -sha256 \
  -verify /tmp/kms-pubkey.pem \
  -signature /tmp/test-sig.bin \
  /tmp/test-digest.bin
```

Output harus:

```text
Verified OK
```

Jika gagal, modulus key yang di-import tidak match dengan `signing.crt`.

---

## 19.5 IAM Policy untuk Service

Buat IAM policy agar `hash-signing-service` bisa memanggil KMS Sign:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "kms:Sign",
        "kms:DescribeKey"
      ],
      "Resource": "arn:aws:kms:ap-southeast-1:<account-id>:key/<KeyId>"
    }
  ]
}
```

Attach ke IAM role EC2/ECS tempat service berjalan.
Jangan pakai access key hardcoded — gunakan IAM role.

---

## 19.6 Konfigurasi hash-signing-service

Update `.env`:

```dotenv
SIGNER_BACKEND=awskms

CERT_FILE=certs/msign/signing.crt
CERT_SUB_CA_FILE=certs/msign/sub-ca.crt
CERT_ROOT_CA_FILE=certs/msign/root-ca.crt

AWS_KMS_REGION=ap-southeast-1
AWS_KMS_KEY_ID=arn:aws:kms:ap-southeast-1:<account-id>:key/<KeyId>
```

`CERT_KEY_FILE` tidak dipakai saat `SIGNER_BACKEND=awskms`.

Credentials AWS diambil dari standard chain secara otomatis:

```text
1. Environment: AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY
2. ~/.aws/credentials
3. IAM Role (EC2 instance profile / ECS task role) ← recommended production
```

---

## 19.7 Perbedaan Teknis KMS vs PKCS#11

```text
PKCS#11 (HSMSigner):
  - Input: raw hash bytes
  - DigestInfo prepend: manual di hsm_signer.go (required CKM_RSA_PKCS)
  - Output: raw RSA signature bytes

AWS KMS (KMSSigner):
  - Input: raw hash bytes (MessageType=DIGEST)
  - DigestInfo prepend: dilakukan KMS secara internal
  - Output: raw RSA signature bytes
```

Karena itu `kms_signer.go` TIDAK prepend DigestInfo, berbeda dengan `hsm_signer.go`.

---

## 19.8 Checklist Sukses AWS KMS

```text
[x] KMS key dibuat dengan key-spec RSA_2048, key-usage SIGN_VERIFY
[x] Key status Enabled
[x] Test signing Verified OK terhadap signing.crt
[x] IAM policy kms:Sign dan kms:DescribeKey terpasang di role service
[x] SIGNER_BACKEND=awskms di .env
[x] AWS_KMS_REGION dan AWS_KMS_KEY_ID terisi
[x] Service restart setelah env berubah
[x] PDF signing end-to-end valid di Adobe
```
