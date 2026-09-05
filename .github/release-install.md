### Install

```bash
# macOS Apple Silicon
gh release download __VERSION__ --repo nbugash-viafoura/clouddesktop --pattern '*darwin-arm64*' --output - | tar xz
sudo mv clouddesktop /usr/local/bin/

# macOS Intel
gh release download __VERSION__ --repo nbugash-viafoura/clouddesktop --pattern '*darwin-amd64*' --output - | tar xz
sudo mv clouddesktop /usr/local/bin/

# Linux
gh release download __VERSION__ --repo nbugash-viafoura/clouddesktop --pattern '*linux-amd64*' --output - | tar xz
sudo mv clouddesktop /usr/local/bin/
```

Verify with `clouddesktop --version`.
