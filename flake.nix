{
  description = "YouTube Audio Transcription TUI with Whisper";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        pkgsCuda = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        whisperCuda = pkgsCuda.whisper-cpp.override { cudaSupport = true; };

        mkPackage = { whisper ? null }: pkgs.buildGoModule {
          pname = "whisper-transcribe";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-sWODEAaAk5j7A/MrKw7lHZj0jRl3IO3Gp7Wh+lgJQNw=";

          nativeBuildInputs = [ pkgs.makeWrapper ];

          postInstall = ''
            wrapProgram $out/bin/whisper-transcribe \
              --prefix PATH : ${pkgs.lib.makeBinPath (
                [ pkgs.yt-dlp pkgs.ffmpeg ]
                ++ pkgs.lib.optional (whisper != null) whisper
              )}
          '';

          meta = with pkgs.lib; {
            description = "TUI for transcribing YouTube videos using Whisper";
            license = licenses.mit;
            mainProgram = "whisper-transcribe";
          };
        };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go development
            go
            gopls
            gotools
            golangci-lint

            # External dependencies
            yt-dlp
            ffmpeg
            whisper-cpp

            # Markdown linting
            markdownlint-cli
          ];

          shellHook = ''
            echo "Whisper Transcription TUI - Development Environment"
            echo ""
            echo "Available tools:"
            echo "  go        - $(go version | cut -d' ' -f3)"
            echo "  yt-dlp    - $(yt-dlp --version)"
            echo "  ffmpeg    - $(ffmpeg -version 2>&1 | head -1 | cut -d' ' -f3)"
            echo "  whisper   - openai-whisper-cpp"
            echo ""
            echo "Install options:"
            echo "  nix profile add .#default  - CPU-only (works everywhere)"
            echo "  nix profile add .#cuda     - NVIDIA GPU acceleration"
            echo "  nix profile add .#npu      - AMD NPU acceleration (requires system flm)"
            echo ""
            echo "Run 'make' to see available commands"
          '';
        };

        packages.default = mkPackage { whisper = pkgs.whisper-cpp; };
        packages.cuda = mkPackage { whisper = whisperCuda; };
        packages.npu = mkPackage {}; # Uses system-provided flm for AMD NPU acceleration
      });
}
