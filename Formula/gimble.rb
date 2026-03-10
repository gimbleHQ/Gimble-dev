class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.2.0"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "0877d6113e72ce5cfa21fdb6b0d35e371f0c1ca5878825128437453c5edaf22f"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.2.0", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
