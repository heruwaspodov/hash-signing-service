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

## 0. Konsep Singkat

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

## 1. Install Package

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

## 2. Setup Config SoftHSM

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

## 3. Init Token SoftHSM

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

## 4. Generate Private Key di SoftHSM

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

## 5. Export Public Key dari SoftHSM

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

## 6. Generate Root CA

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

## 7. Generate Sub CA

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

## 8. Generate CSR Signing Certificate dari Key SoftHSM

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

## 9. Generate Signing Certificate dari Sub CA

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

## 10. Buat Certificate Chain

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

## 11. Verifikasi Public Key Signing Certificate Sama dengan SoftHSM

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

## 12. File yang Dihasilkan

### Private Key

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

### Public Key

```text
public.der
public.pem
```

### CSR

```text
sub-ca.csr
signing.csr
```

### Certificate

```text
root-ca.crt
sub-ca.crt
signing.crt
```

### Chain

```text
chain.crt
fullchain.crt
```

### Extension Config

```text
sub-ca.ext
signing.ext
```

### Serial Number

```text
root-ca.srl
sub-ca.srl
```

---

## 13. Untuk HexaPDF / PDF Signing

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

## 14. Membuat Signed By Berbeda dengan Key yang Sama

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

## 16. Catatan Adobe Trust

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
