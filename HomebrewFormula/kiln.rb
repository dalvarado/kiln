# This file was generated by GoReleaser. DO NOT EDIT.
class Kiln < Formula
  desc ""
  homepage ""
  version "0.44.0"
  bottle :unneeded

  if OS.mac?
    url "https://github.com/dalvarado/kiln/releases/download/0.44.0/kiln-darwin-0.44.0.tar.gz"
    sha256 "c6d83d86a5c5cdb1d474a1a91615addc28903eb44004faebed929e5742d85dba"
  elsif OS.linux?
    if Hardware::CPU.intel?
      url "https://github.com/dalvarado/kiln/releases/download/0.44.0/kiln-linux-0.44.0.tar.gz"
      sha256 "01a752112e9928492e1f2f59e971f6026d6196a09166731eefd1710f24bb9c56"
    end
  end

  def install
    bin.install "kiln"
  end

  test do
    system "#{bin}/kiln --version"
  end
end
