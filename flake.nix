{
  description = "structlua - Go library and tools to map Lua tables to and from Go structs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGo127Module {
          pname = "structlua";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          subPackages = [ "." ];
          env.GOWORK = "off";
        };
      });

      formatter = forAllSystems (
        pkgs:
        pkgs.writeShellScriptBin "formatter" ''
          ${pkgs.go_1_27}/bin/gofmt -w "''${PRJ_ROOT:-.}"
          exec ${pkgs.nixfmt-tree}/bin/treefmt "$@"
        ''
      );

      apps = forAllSystems (
        pkgs:
        let
          testRunner = pkgs.writeShellScript "test-runner" ''
            exec ${pkgs.go_1_27}/bin/go test -v ./... ./golua/... ./gopherlua/... "$@"
          '';
          testApp = {
            type = "app";
            program = toString testRunner;
          };
        in
        {
          default = testApp;
          test = testApp;
        }
      );

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_27
            gopls
            gotools
          ];
        };
      });
    };
}
