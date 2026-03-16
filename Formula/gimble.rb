class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.5.9"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.5.9.tar.gz"
  sha256 "1a46779e4ea489425e7cf13296f050bfc9855c20a81d632dc25514e649889433"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.5.9", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
