class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.2"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.2.tar.gz"
  sha256 "c02f8f5032580b45261de8944bd7bb7d70a24285a5b6ad35158142c4b19bbea0"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.2", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
