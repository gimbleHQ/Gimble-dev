class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.6.tar.gz"
  sha256 "4a31ae4a1bc1585a54446d34ec2843a18e4063cc72f3894a71ff17e846bd894b"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.6", "-o", bin/"gimble", "./cmd/gimble"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
