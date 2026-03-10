class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.0"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "5f25128675815e4b6eca3c28ac48615e0ed5ea45ccd3e839df110a57f5495630"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
