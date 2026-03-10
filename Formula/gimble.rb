class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.0"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.0.tar.gz"
  sha256 "53f0e507577a665ee9725e4a797b20af038b62489590812710cc8d50c6e5f3ed"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
