import React, { useState, useEffect } from 'react';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8081';
const BOT_ID = process.env.REACT_APP_BOT_ID;

function App() {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        console.log('Bot ID:', BOT_ID);

        const token = localStorage.getItem('token');
        if (token) {
            verifyToken(token);
        } else {
            setLoading(false);
        }
    }, []);

    const verifyToken = async (token) => {
        try {
            const response = await fetch(`${API_URL}/api/me`, {
                headers: {
                    'Authorization': `Bearer ${token}`
                }
            });

            if (response.ok) {
                const data = await response.json();
                console.log('User data from token:', data);
                setUser({
                    role: data.role,
                    username: data.username,
                    userId: data.userId
                });
            } else {
                localStorage.removeItem('token');
            }
        } catch (err) {
            console.error('Error:', err);
        } finally {
            setLoading(false);
        }
    };

    const handleTelegramLogin = () => {
        if (!BOT_ID) {
            setError('Bot ID not configured');
            return;
        }

        const redirectUrl = window.location.origin;
        const authUrl = `https://oauth.telegram.org/auth?bot_id=${BOT_ID}&origin=${encodeURIComponent(redirectUrl)}&embed=1&request_access=write`;

        const width = 600;
        const height = 700;
        const left = (window.innerWidth - width) / 2;
        const top = (window.innerHeight - height) / 2;

        const authWindow = window.open(
            authUrl,
            'Telegram Auth',
            `width=${width},height=${height},left=${left},top=${top}`
        );

        if (!authWindow) {
            setError('Please allow popups for this site');
            return;
        }

        const handleMessage = (event) => {
            console.log('Message from:', event.origin, event.data);

            if (event.origin === 'https://oauth.telegram.org') {
                console.log('Full message data:', JSON.stringify(event.data));

                // Проверяем разные форматы данных
                let userData = null;

                if (event.data && event.data.hash) {
                    userData = event.data;
                } else if (event.data && event.data.user) {
                    userData = event.data.user;
                } else if (event.data && typeof event.data === 'object') {
                    userData = event.data;
                }

                if (userData && userData.hash) {
                    console.log('Got user data:', userData);
                    authWindow.close();
                    sendAuthToBackend(userData);
                    window.removeEventListener('message', handleMessage);
                }
            }
        };

        window.addEventListener('message', handleMessage);

        setTimeout(() => {
            window.removeEventListener('message', handleMessage);
        }, 300000);
    };

    const sendAuthToBackend = async (authData) => {
        console.log('Sending to backend:', authData);
        setLoading(true);
        setError(null);

        try {
            const payload = {
                id: authData.id,
                first_name: authData.first_name,
                last_name: authData.last_name || '',
                username: authData.username || '',
                photo_url: authData.photo_url || '',
                auth_date: authData.auth_date,
                hash: authData.hash
            };

            console.log('Payload to backend:', payload);

            const response = await fetch(`${API_URL}/api/auth`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(payload)
            });

            const data = await response.json();
            console.log('Backend response:', data);

            if (data.success) {
                localStorage.setItem('token', data.token);
                setUser({
                    role: data.role,
                    username: authData.username || authData.first_name,
                    userId: authData.id
                });
                console.log('User set successfully:', data.role);
            } else {
                setError(data.message || 'Authentication failed');
            }
        } catch (err) {
            setError('Network error. Check if backend is running.');
            console.error('Auth error:', err);
        } finally {
            setLoading(false);
        }
    };

    const handleLogout = () => {
        localStorage.removeItem('token');
        setUser(null);
    };

    const getGreeting = () => {
        if (!user) return 'Привет, гость';
        if (user.role === 'admin') return `Привет, админ ${user.username || ''}! ✨`;
        return `Привет, пользователь ${user.username || ''}! 👋`;
    };

    if (loading) {
        return (
            <div style={{ textAlign: 'center', marginTop: '50px' }}>
                <h1>🔐 Telegram Auth App</h1>
                <div>Загрузка...</div>
            </div>
        );
    }

    return (
        <div style={{ textAlign: 'center', marginTop: '50px', fontFamily: 'Arial, sans-serif' }}>
            <h1>🔐 Telegram Auth App</h1>

            <div style={{
                fontSize: '24px',
                margin: '30px',
                padding: '20px',
                backgroundColor: '#f5f5f5',
                borderRadius: '8px'
            }}>
                {getGreeting()}
            </div>

            {error && (
                <div style={{
                    color: '#dc3545',
                    margin: '10px',
                    padding: '10px',
                    backgroundColor: '#ffe6e6',
                    borderRadius: '4px'
                }}>
                    {error}
                </div>
            )}

            {!user ? (
                <div>
                    <button
                        onClick={handleTelegramLogin}
                        style={{
                            padding: '12px 24px',
                            fontSize: '16px',
                            backgroundColor: '#0088cc',
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            cursor: 'pointer'
                        }}
                    >
                        Войти через Telegram
                    </button>
                    <p style={{ marginTop: '20px', fontSize: '14px', color: '#666' }}>
                        Нажмите кнопку для входа через Telegram
                    </p>
                    <p style={{ fontSize: '12px', color: '#999' }}>
                        Bot ID: {BOT_ID}
                    </p>
                </div>
            ) : (
                <div>
                    <div style={{
                        margin: '20px',
                        padding: '15px',
                        backgroundColor: '#e9ecef',
                        borderRadius: '4px'
                    }}>
                        <p><strong>👤 Роль:</strong> {user.role === 'admin' ? 'Администратор' : 'Пользователь'}</p>
                        {user.username && <p><strong>📝 Имя:</strong> {user.username}</p>}
                        {user.userId && <p><strong>🆔 ID:</strong> {user.userId}</p>}
                    </div>
                    <button
                        onClick={handleLogout}
                        style={{
                            padding: '10px 20px',
                            fontSize: '16px',
                            backgroundColor: '#dc3545',
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            cursor: 'pointer'
                        }}
                    >
                        Выйти
                    </button>
                </div>
            )}
        </div>
    );
}

export default App;