import { Shell } from "./components/ui";
import { useAuth } from "./lib/auth";
import { useRoute } from "./lib/router";
import { AuthScreen } from "./screens/AuthScreen";
import { GameScreen } from "./screens/GameScreen";
import { LobbyScreen } from "./screens/LobbyScreen";

export function App() {
  const { auth } = useAuth();
  const route = useRoute();

  if (!auth) {
    return <AuthScreen invited={route.name === "join"} />;
  }

  return (
    <Shell>
      {route.name === "game" ? (
        <GameScreen key={route.id} id={route.id} />
      ) : (
        <LobbyScreen initialCode={route.name === "join" ? route.code : undefined} />
      )}
    </Shell>
  );
}
