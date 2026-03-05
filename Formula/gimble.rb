class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.14"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.14.tar.gz"
  sha256 "fd437955ddf0d132c28b77bc940ed035478ced338a9a819f77b8bbbfd1810c04"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.14", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
