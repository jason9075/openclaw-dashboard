{
  description = "OpenClaw Dashboard - specific development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            air
            gopls
            gotools
            golangci-lint
            just
            nodejs_22 # For any frontend tooling if needed later, though vanilla is preferred
            fswatch
            xdotool
          ];

          shellHook = ''
            echo "Welcome to OpenClaw Dashboard Dev Environment"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
