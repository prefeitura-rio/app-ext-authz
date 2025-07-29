{
  description = "Envoy External Authorization Service for reCAPTCHA";

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
            go_1_24
            gcc
            docker
            docker-compose
            redis
            curl
            jq
            just
            delve
            gopls
            go-tools
            gotools
            air
            golangci-lint
          ];

          shellHook = ''
            echo "🚀 Envoy reCAPTCHA Authz Development Environment"
            echo "Available commands:"
            echo "  just run      - Run the service locally"
            echo "  just test     - Run tests"
            echo "  just build    - Build the binary"
            echo "  just docker   - Build and run with Docker"
            echo "  just load     - Run load tests"
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "recaptcha-authz";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

          meta = with pkgs.lib; {
            description = "Envoy External Authorization Service for reCAPTCHA";
            homepage = "https://github.com/prefeitura-rio/app-ext-authz";
            license = licenses.mit;
            platforms = platforms.linux ++ platforms.darwin;
          };
        };
      }
    );
} 