export const LoginPage = () => {
  const handleLogin = () => {
    const clientId = import.meta.env.VITE_DISCORD_CLIENT_ID;
    const redirectUri = encodeURIComponent(import.meta.env.VITE_DISCORD_REDIRECT_URI);
    const scope = encodeURIComponent('identify'); // 必要に応じて email 等追加

    // Discord OAuth URL構築
    const discordUrl = `https://discord.com/api/oauth2/authorize?client_id=${clientId}&redirect_uri=${redirectUri}&response_type=code&scope=${scope}`;
    
    window.location.href = discordUrl;
  };

  return (
    <div className="flex flex-col items-center justify-center h-screen bg-gray-100">
      <div className="p-8 bg-white rounded shadow-md">
        <h1 className="mb-6 text-2xl font-bold text-center">HSS Science Login</h1>
        <button
          onClick={handleLogin}
          className="px-6 py-2 text-white bg-indigo-600 rounded hover:bg-indigo-700 transition"
        >
          Login with Discord
        </button>
      </div>
    </div>
  );
};