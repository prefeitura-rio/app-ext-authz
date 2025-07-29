{
  description = "Envoy External Authorization Service for reCAPTCHA Enterprise";

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
            golangci-lint
            grpcurl
          ];

          shellHook = ''
            echo "🚀 Envoy reCAPTCHA Enterprise Authz Development Environment"
            echo "Available commands:"
            echo "  just run         - Run the service locally"
            echo "  just run-mock    - Run in mock mode"
            echo "  just build       - Build the binary"
            echo "  just docker-compose - Run with Docker Compose"
            echo "  just test-curl-http  - Test HTTP endpoint"
            echo "  just test-curl-grpc  - Test gRPC endpoint"
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "recaptcha-authz";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

          meta = with pkgs.lib; {
            description = "Envoy External Authorization Service for reCAPTCHA Enterprise";
            homepage = "https://github.com/prefeitura-rio/app-ext-authz";
            license = licenses.mit;
            platforms = platforms.linux ++ platforms.darwin;
          };
        };
      }
    );
} 