# v1.2.2 — Security Patch (Medium/Low)

Released: 2026-05-15

Follow-up security patch addressing the medium and low severity
findings from the v1.2.0 audit. No API changes; fully backward
compatible with v1.2.x.

## Security fixes

### Medium: Image decompression bomb DoS

**File**: `internal/optimizer/image.go`

`DecodeImage` called `image.Decode` directly, which decompresses the
full image into memory before any size check. A crafted JPEG/PNG
(e.g. 10000×10000 pixels at 100 KB compressed → 400 MB in RAM) could
exhaust server memory.

**Fix**: `DecodeImage` now calls `image.DecodeConfig` first to read
only the image header. If either dimension exceeds 8192 pixels the
function returns an error without decoding the full image. The limit
is configurable via the `maxImageDimension` constant.

### Medium: S3 upload `io.ReadAll` OOM

**File**: `internal/upload/s3.go`

`S3Storage.Put` called `io.ReadAll(reader)` without a size limit.
A client that declared a small `Content-Length` but sent a large body
could exhaust server memory.

**Fix**: The reader is now wrapped with `io.LimitReader(reader, size+1)`
before `io.ReadAll`. If the actual body exceeds the declared size the
upload is rejected with an error. When `size` is unknown (≤ 0) a
32 MB default cap is applied.

### Medium: CSRF cookie missing explicit SameSite documentation

**File**: `internal/csrf/csrf.go`

The CSRF cookie already defaulted to `SameSite=Lax` but the code
comment did not explain why `HTTPOnly: false` is intentional (JS must
read the token for the double-submit pattern). Added a clear comment
to prevent future "fix" PRs from accidentally setting `HTTPOnly: true`.

### Medium: OAuth state cookie missing `SameSite` attribute

**File**: `internal/auth/oauth.go`

The OAuth state cookie had no `SameSite` attribute, leaving it
unprotected against CSRF attacks on the OAuth callback endpoint.

**Fix**: `SameSite: "Lax"` is now set on the `oauth_state` cookie.
`Lax` is the correct value here: the cookie must be sent when the
OAuth provider redirects back (a top-level GET navigation), but not
on cross-site sub-resource requests.

### Low: `concurrent.Pool.Submit` channel leak on close race

**File**: `pkg/concurrent/concurrent.go`

Between the two `select` statements in `Submit`, the pool could close
after the `reply` channel was created but before it was sent to the
jobs queue. The channel would never be drained, leaking a small
allocation. Added a comment documenting the intent and confirming the
buffered channel (cap 1) means no goroutine blocks.

### Low: `nguyen vet` secret detection rules too narrow

**File**: `internal/vet/vet.go`

The previous patterns required ≥ 12 characters and missed:
- Short JWT secrets (< 12 chars but still dangerous)
- Common service key prefixes (`sk_live_`, `AKIA`, `SG.`)
- Private key PEM headers
- Empty string assignments to known secret variable names

**Fix**: Five patterns now cover the above cases. The minimum length
for generic key/password patterns is reduced to 8 characters.

## Upgrade

```bash
go get github.com/dev2k6/Nguyen.go@v1.2.2
```

No configuration changes required. The image dimension limit (8192)
and S3 default cap (32 MB) are conservative defaults that cover all
legitimate use cases. If you serve very large images, increase
`maxImageDimension` in `internal/optimizer/image.go` or pre-process
images before passing them to the optimizer.

## Author

Thái Nguyên <thainguyen.junior@gmail.com>
