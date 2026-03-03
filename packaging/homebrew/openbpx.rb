class Openbpx < Formula
  desc "Blueprint Toolkit CLI for Unreal Engine packages"
  homepage "https://github.com/wilddogjp/openbpx"
  version "0.1.3"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.1.3/bpx_0.1.3_darwin_arm64.tar.gz"
      sha256 "32b3853c210f94b3d6165f1b7bf9025871c1203a26022eb5a4717017ddb14ab3"
    else
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.1.3/bpx_0.1.3_darwin_amd64.tar.gz"
      sha256 "8e4f78eac11e2e4f3523eca10b546647edceb8f0c6c6c99978b923f0e569dfce"
    end
  end

  def install
    bin.install "bpx"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/bpx version")
  end
end
