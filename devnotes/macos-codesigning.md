# macOS Code-Signing via Pycodesign

*2026-02-13T17:41:17Z*

Added Apple code-signing, notarization, and .pkg installer generation to pm using Pycodesign -- the same pipeline used in dbsnapper/agent. macOS Gatekeeper no longer flags the binary, and users can install via a signed .pkg or Homebrew Cask.

## What Changed

Four files were modified or created:

1. **pm_pycodesign.ini** (new) -- Pycodesign configuration with certificate fingerprints for Developer ID Application and Installer certs (renewed Aug 2025, valid until 2030).
2. **.goreleaser.yml** (modified) -- Split the single multi-platform build into three (linux, macos, windows). Added universal_binaries with a pycodesign post-hook that signs, packages, notarizes, and staples the macOS binary. Split archives by OS format (tar.gz for Linux, zip for macOS/Windows). Added release section with the .pkg as an extra file.
3. **Makefile** (modified) -- Added `release-local` target for signed local releases.
4. **.github/workflows/release.yml** (modified) -- Changed from automatic tag-push trigger to manual workflow_dispatch, since releases now run locally with code-signing (CI on Ubuntu cannot sign).

## Certificate Details

- **Application**: `35B7E2AD4300183575B03C4A7D1F08CE01E1BEC4` (Developer ID Application: Scharfnado LLC)
- **Installer**: `AA10A9BE083F6EBB184741D02064545105989B9F` (Developer ID Installer: Scharfnado LLC)
- **Keychain profile**: SCHARFNADO_LLC
- **Valid until**: Aug 12, 2030

## Verification: GoReleaser Config

```bash
goreleaser check
```

```output
  • checking                                  path=.goreleaser.yml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
```

## Verification: Signed Binary

```bash
codesign -dv --verbose=4 dist/pm-macos_darwin_all/pm 2>&1 | head -20
```

```output
Executable=/Users/joescharf/app/pm.worktrees/feat-codesign/dist/pm-macos_darwin_all/pm
Identifier=pm
Format=Mach-O universal (x86_64 arm64)
CodeDirectory v=20500 size=42446 flags=0x10000(runtime) hashes=1321+2 location=embedded
VersionPlatform=1
VersionMin=786432
VersionSDK=786432
Hash type=sha256 size=32
CandidateCDHash sha256=c66e8567b6ab4b023c20b15c0a9492d0299035ea
CandidateCDHashFull sha256=c66e8567b6ab4b023c20b15c0a9492d0299035ea226d86aef26a69c6622466b3
Hash choices=sha256
CMSDigest=c66e8567b6ab4b023c20b15c0a9492d0299035ea226d86aef26a69c6622466b3
CMSDigestType=2
Executable Segment base=0
Executable Segment limit=18726912
Executable Segment flags=0x1
Page size=16384
CDHash=c66e8567b6ab4b023c20b15c0a9492d0299035ea
Signature size=9049
Authority=Developer ID Application: Scharfnado LLC (PC9WL4QUXV)
```

## Verification: Notarized .pkg

```bash
pkgutil --check-signature dist/pm_macos_universal.pkg 2>&1
```

```output
Package "pm_macos_universal.pkg":
   Status: signed by a developer certificate issued by Apple for distribution
   Notarization: trusted by the Apple notary service
   Signed with a trusted timestamp on: 2026-02-13 17:39:53 +0000
   Certificate Chain:
    1. Developer ID Installer: Scharfnado LLC (PC9WL4QUXV)
       Expires: 2030-08-12 18:27:19 +0000
       SHA256 Fingerprint:
           42 22 D6 89 78 98 39 81 C8 53 BB CA 3D 80 74 84 2C F3 F2 8B 27 18 
           0A 4C 42 7E 32 00 41 C2 9E 72
       ------------------------------------------------------------------------
    2. Developer ID Certification Authority
       Expires: 2031-09-17 00:00:00 +0000
       SHA256 Fingerprint:
           F1 6C D3 C5 4C 7F 83 CE A4 BF 1A 3E 6A 08 19 C8 AA A8 E4 A1 52 8F 
           D1 44 71 5F 35 06 43 D2 DF 3A
       ------------------------------------------------------------------------
    3. Apple Root CA
       Expires: 2035-02-09 21:40:36 +0000
       SHA256 Fingerprint:
           B0 B1 73 0E CB C7 FF 45 05 14 2C 49 F1 29 5E 6E DA 6B CA ED 7E 2C 
           68 C5 BE 91 B5 A1 10 01 F0 24

```

## Verification: Release Artifacts

```bash
ls -lh dist/*.{zip,tar.gz,pkg,txt} 2>&1
```

```output
-rw-r--r-- 1 joescharf staff  436 Feb 13 10:40 dist/checksums.txt
-rw-r--r-- 1 joescharf staff  16M Feb 13 10:40 dist/pm_Darwin_all.zip
-rw-r--r-- 1 joescharf staff 7.2M Feb 13 10:40 dist/pm_Linux_arm64.tar.gz
-rw-r--r-- 1 joescharf staff 7.8M Feb 13 10:40 dist/pm_Linux_x86_64.tar.gz
-rw-r--r-- 1 joescharf staff  16M Feb 13 10:40 dist/pm_macos_universal.pkg
-rw-r--r-- 1 joescharf staff 7.3M Feb 13 10:40 dist/pm_Windows_arm64.zip
-rw-r--r-- 1 joescharf staff 8.0M Feb 13 10:40 dist/pm_Windows_x86_64.zip
```

## Release Workflow

With this change, the release workflow is:

1. Tag: `git tag v0.x.y && git push --tags`
2. Build + sign locally: `make release-local`
3. GoReleaser builds all platforms, creates universal macOS binary, signs/notarizes/staples via pycodesign, publishes draft release to GitHub with .pkg as extra artifact, and updates Homebrew Cask.
4. CI workflow is now manual-only (workflow_dispatch) as a fallback for unsigned releases.
