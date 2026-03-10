class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.9"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.9.tar.gz"
  sha256 "9f204b943f1d23a06d08bfa95894a331da29fb4378cb99a0a477d743d116cd79"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.9", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
