@echo off
setlocal

set "ARCH=%PROCESSOR_ARCHITECTURE%"
if /I "%ARCH%"=="AMD64" set "ARCH=amd64"
if /I "%ARCH%"=="ARM64" set "ARCH=arm64"

if not "%ARCH%"=="amd64" if not "%ARCH%"=="arm64" (
  echo unsupported architecture for bundled bpx: %PROCESSOR_ARCHITECTURE% 1>&2
  exit /b 1
)

set "BPX_EXE=%~dp0..\plugin-bin\bpx-windows-%ARCH%.exe"
if not exist "%BPX_EXE%" (
  echo bundled bpx binary not found: %BPX_EXE% 1>&2
  exit /b 1
)

"%BPX_EXE%" %*
exit /b %ERRORLEVEL%
