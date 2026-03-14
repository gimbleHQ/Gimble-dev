class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.7"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.7.tar.gz"
  sha256 "daf9cab985ea3be917d9bbc6c23545c9f8d942f887b058ba3532c71773f1fe80"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.7", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
