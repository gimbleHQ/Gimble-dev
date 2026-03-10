class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.6.tar.gz"
  sha256 "974fb91ae8e9a0cd88c4a185b59ab02d829fb815b022ecccd1e39146d24cebc0"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.6", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
