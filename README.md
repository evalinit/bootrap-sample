````markdown
# bootrap-sample

Tiny test app for the **Bootrap** launcher. It:
- Listens on a TCP address (argv[1], default `127.0.0.1:59595`)
- Creates a user data dir and writes `hello.txt`
- Appends each received NDJSON line to `messages.log`

## Downloads
Artifacts and manifest:
- Manifest: https://<owner>.github.io/<repo>/manifest.json
- Files:    https://<owner>.github.io/<repo>/dist/

Filenames: `<version>_<os-arch>.{zip|tar.gz}` (contains `bin/bootrap-sample[.exe]`).

## Run without the launcher

### Linux (amd64 example)
```bash
VER=1.0.0
curl -LO "https://<owner>.github.io/<repo>/dist/${VER}_linux-amd64.tar.gz"
tar -xzf "${VER}_linux-amd64.tar.gz"
./bin/bootrap-sample 127.0.0.1:59595
# In another shell, send a deeplink message:
printf '%s\n' '{"type":"deeplink","url":"yourapp://join?room=alpha","mapped":{"mode":"raw_url","url":"yourapp://join?room=alpha"}}' | nc 127.0.0.1 59595
````

### Windows (amd64 example, PowerShell)

```powershell
$ver="1.0.0"
Invoke-WebRequest "https://<owner>.github.io/<repo>/dist/${ver}_windows-amd64.zip" -OutFile sample.zip
Expand-Archive sample.zip -DestinationPath .\unpacked -Force
.\unpacked\bin\bootrap-sample.exe 127.0.0.1:59595
# In another PS window:
$tcp=new-object Net.Sockets.TcpClient("127.0.0.1",59595)
$sw=new-object IO.StreamWriter($tcp.GetStream());$sw.NewLine="`n"
$sw.WriteLine('{"type":"deeplink","url":"yourapp://join?room=alpha","mapped":{"mode":"raw_url","url":"yourapp://join?room=alpha"}}');$sw.Flush();$tcp.Close()
```

## Data directory

* Linux: `${XDG_STATE_HOME:-$HOME/.local/state}/Your Desktop App/`
* macOS: `~/Library/Application Support/Your Desktop App/`
* Windows: `%LOCALAPPDATA%\Your Desktop App\`

Check `hello.txt` and `messages.log` there.

## Using with Bootrap

In Bootrap’s `config.yaml`:

```yaml
child_binary_name: "bootrap-sample"
manifest_url: "https://<owner>.github.io/<repo>/manifest.json"
public_key_pem: |  # paste your Ed25519 public key
  -----BEGIN PUBLIC KEY-----
  ...
  -----END PUBLIC KEY-----
url_scheme: "yourapp"
```