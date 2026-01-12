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
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go development
            go_1_22
            gopls
            gotools
            golangci-lint

            # External dependencies
            yt-dlp
            ffmpeg
            openai-whisper-cpp

            # Markdown linting
            nodePackages.markdownlint-cli
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
            echo "Run 'make' to see available commands"
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "whisper-transcribe";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;

          nativeBuildInputs = [ pkgs.makeWrapper ];

          postInstall = ''
            wrapProgram $out/bin/whisper-transcribe \
              --prefix PATH : ${pkgs.lib.makeBinPath [
                pkgs.yt-dlp
                pkgs.ffmpeg
                pkgs.openai-whisper-cpp
              ]}
          '';

          meta = with pkgs.lib; {
            description = "TUI for transcribing YouTube videos using Whisper";
            license = licenses.mit;
            mainProgram = "whisper-transcribe";
          };
        };
      });
}
