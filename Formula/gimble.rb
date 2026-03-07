class Gimble < Formula
  desc "Gimble CLI"
  homepage "https://github.com/Saketspradhan/Gimble-dev"
  version "0.1.22"
  url "https://github.com/Saketspradhan/Gimble-dev/archive/refs/tags/v0.1.22.tar.gz"
  sha256 "bbaf277f011d9d9186d9e82711863282421fdca32247c8cf47636c2201f9522e"
  license "MIT"

  depends_on "go" => :build
  depends_on "python@3.12"

  def install
    system "go", "build", "-ldflags", "-X main.version=0.1.22", "-o", bin/"gimble", "./cmd/gimble"
    pkgshare.install "python"
  end

  test do
    assert_match "gimble", shell_output("#{bin}/gimble --version")
  end
end
