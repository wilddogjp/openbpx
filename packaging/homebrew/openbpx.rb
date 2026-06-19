class Openbpx < Formula
  desc "Blueprint Toolkit CLI for Unreal Engine packages"
  homepage "https://github.com/wilddogjp/openbpx"
  version "0.2.1"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.2.1/bpx-darwin-arm64", using: :nounzip
      sha256 "2a000c6eafc95a0eff295fa05f9ee1efaf728e2c3558531d82378e34d384e7d2"
    else
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.2.1/bpx-darwin-amd64", using: :nounzip
      sha256 "fb8096047e89ddc81e3daa6548f2cd5b090add5e8d71b26c1434e5af8b271eed"
    end
  end

  def install
    bin.install cached_download => "bpx"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/bpx version")
  end
end
