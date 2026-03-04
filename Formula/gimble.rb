class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.6"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.6.tar.gz"
  sha256 "890b877054b6dc09fb3b4619d80c815fca6a9bfb4f2e1165dec3f4c107439f8d"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.6", "-o", bin/"gimble", "./cmd/gimble"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
