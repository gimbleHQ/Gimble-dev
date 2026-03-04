class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.4"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.4.tar.gz"
  sha256 "9bac0053877e79bb9d4f1e57e1ef17f41daf42a3ae35f1b5417a74248fa93b9e"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.4", "-o", bin/"gimble", "./cmd/gimble"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
