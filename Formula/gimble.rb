class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.3.2"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.3.2.tar.gz"
  sha256 "13f6ef396eb7a49ff2d234a6344beaadf3087589f884e89ce8402c181b5a673a"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.3.2", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
