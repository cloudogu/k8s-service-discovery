# Wartungsmodus

Dieses Dokument erklärt den Wartungsmodus und wie man diesen für das Cloudogu EcoSystem MultiNode steuern kann.

Der Wartungsmodus ist ein Systemzustand des Ecosystem, bei dem ein externer Zugriff auf das EcoSystem deaktiviert wird.
Der Modus wird benötigt, wenn systemkritische Prozesse laufen. Während der Wartungsmodus aktiviert ist, wird für jeden
Zugriff auf Dogus eine Wartungsseite angezeigt.

# Wartungsmodus aktivieren

Um das CES in den Wartungsmodus zu versetzen, muss der folgende String in `/config/_global/maintenance` geschrieben
werden:

```json
{
  "title": "Dies ist der Titel",
  "text": "Das ist der Text"
}
``` 

Jede Anfrage an den CES wird dann mit dem HTTP-Code 503 (Service Unavailable) beantwortet, bis der Schlüssel im etcd (
s.o.) gelöscht wird. Dabei wird auf der Seite der Inhalt von `title` und `text` angezeigt.

**Hinweis:** Das Aktivieren und Deaktivieren des Wartungsmodus führt zu einem Neustart des Nginx-Static Dogus. Dies
sollte jedoch nur wenige Sekunden in Anspruch nehmen.

## Vorsicht

Da die Wartungsseite von nginx bedient wird, ist es nicht möglich, die Wartungsmodus-Seite anzuzeigen, während ein
Upgrade von Nginx läuft.