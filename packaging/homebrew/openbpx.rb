class Openbpx < Formula
  desc "Blueprint Toolkit CLI for Unreal Engine packages"
  homepage "https://github.com/wilddogjp/openbpx"
  version "0.2.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.2.0/bpx-darwin-arm64", using: :nounzip
      sha256 "bf2948219abf20865045ff6f928cdec92a2a3e27eb8a9e08d71295092a59a11b"
    else
      url "https://github.com/wilddogjp/openbpx/releases/download/v0.2.0/bpx-darwin-amd64", using: :nounzip
      sha256 "59d2038992c0258c744615c6f71b498b56e85074f3ac57666794e952fe47bfc0"
    end
  end

  def install
    bin.install cached_download => "bpx"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/bpx version")
  end
end
