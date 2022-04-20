# Leitfaden zur Entwicklung

## Lokale Deploy

1. Die Umgebungsvariable KUBECONFIG muss eine valide Konfig für das Zielcluster verfügen.
1. Exportieren sie die Umgebungsvariable `WATCH_NAMESPACE`: `export WATCH_NAMESPACE=ecosystem`. 
1. Führen Sie `make run` aus, um den Service Discovery-Operator lokal auszuführen.

## Makefile-Targets

Der Befehl `make help` gibt alle verfügbaren Targets und deren Beschreibungen in der Kommandozeile aus.