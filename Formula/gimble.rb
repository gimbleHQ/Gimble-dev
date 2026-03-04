class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.2.tar.gz"
  sha256 "8262698522ad8a7dcf033ac815dc0c1f4132c0f3b1fae23ab4a8dbd1055a8da6"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.2", "-o", bin/"gimble", "./cmd/gimble"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
