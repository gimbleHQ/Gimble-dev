class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.4"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.4.tar.gz"
  sha256 "c8678f49fb97d36fb4d9602e5498162f46ef02cd5bb3a56aeb63e79759e7a2e7"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.4", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
