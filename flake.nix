{
  description = "Metrics Server Operator - A Kubernetes operator for deploying metrics-server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
        # Go version to match the project requirements
        go = pkgs.go_1_23;
        
        # Kubernetes tools
        kubernetes-tools = with pkgs; [
          kubectl
          kind
          kubernetes-helm
          kustomize
        ];

        # Development tools
        dev-tools = with pkgs; [
          # Core development
          go
          gnumake
          git
          
          # Container tools
          docker
          docker-compose
          
          # Kubernetes development
          operator-sdk
          
          # Code quality and analysis
          golangci-lint
          gosec
          govulncheck
          
          # Testing tools
          gotestsum
          ginkgo
          
          # Documentation and formatting
          markdownlint-cli
          nodePackages.prettier
          
          # Shell and utilities
          jq
          yq-go
          curl
          wget
          
          # Editor support
          gopls
          delve
        ];

        # All tools combined
        allTools = dev-tools ++ kubernetes-tools;

      in
      {
        # Development shell
        devShells.default = pkgs.mkShell {
          buildInputs = allTools;
          
          shellHook = ''
            echo "🚀 Metrics Server Operator Development Environment"
            echo "=================================================="
            echo ""
            echo "Available tools:"
            echo "  Go:           $(go version | cut -d' ' -f3-4)"
            echo "  kubectl:      $(kubectl version --client --short 2>/dev/null || echo 'not connected')"
            echo "  kind:         $(kind --version)"
            echo "  operator-sdk: $(operator-sdk version)"
            echo "  golangci-lint: $(golangci-lint --version | head -1)"
            echo ""
            echo "Quick start commands:"
            echo "  make help     - Show all available make targets"
            echo "  make test     - Run unit tests"
            echo "  make build    - Build the operator binary"
            echo "  make run      - Run the operator locally"
            echo ""
            echo "Kind cluster commands:"
            echo "  kind create cluster --name metrics-server-test"
            echo "  kind delete cluster --name metrics-server-test"
            echo ""
            
            # Set up Go environment
            export GOPROXY=https://proxy.golang.org,direct
            export GOSUMDB=sum.golang.org
            export CGO_ENABLED=0
            
            # Set up development environment variables
            export KUBEBUILDER_ASSETS="${pkgs.kubebuilder}/bin"
            
            # Ensure go modules are available
            if [ -f go.mod ]; then
              echo "Installing Go dependencies..."
              go mod download
            fi
          '';

          # Environment variables
          GO = "${go}/bin/go";
          GOROOT = "${go}/share/go";
        };

        # Formatter for nix files
        formatter = pkgs.nixpkgs-fmt;

        # Package the operator
        packages.default = pkgs.buildGoModule {
          pname = "metrics-server-operator";
          version = "latest";
          
          src = ./.;
          
          vendorHash = null; # Update this with the actual vendor hash when needed
          
          buildInputs = [ go ];
          
          meta = with pkgs.lib; {
            description = "Kubernetes operator for deploying metrics-server";
            homepage = "https://github.com/vexxhost/metrics-server-operator";
            license = licenses.asl20;
            maintainers = [ ];
          };
        };

        # Apps for running the operator
        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        # Development checks
        checks = {
          # Format check
          format-check = pkgs.runCommand "format-check" { 
            buildInputs = [ pkgs.nixpkgs-fmt ]; 
          } ''
            cd ${./.}
            nixpkgs-fmt --check flake.nix
            touch $out
          '';
          
          # Go vet check
          go-vet = pkgs.runCommand "go-vet" {
            buildInputs = [ go ];
          } ''
            cd ${./.}
            go vet ./...
            touch $out
          '';
        };
      });
}