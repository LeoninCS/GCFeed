import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const TOKEN_KEY = "gcfeed.accessToken";
const USER_KEY = "gcfeed.user";

const image = {
  currentUser:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuAEH5ZNpPdQoO7Qiy3CGshEypK0dp1HFeVoZ1TAHDLhfcvYMg_js-k2rhBTIPpqMjs6GQpIgKMnUhIu0tY_QYUCTocPD70FDbGWYmHI25NPQ1Quod_7Ssaq0ICv7vvwNephDLNouriPhG7ubVy8GZKbFP9Qi-2yLe2mzST0t9Vfygo2jvgiHITh11LVRZ2ZTcE3nmySn6ZMnpqONtz0zbaKbQMLsDNfR-5smwYHCQLvdp6U5U2-OW_kZS1P6U9vR_PN9Ey84a1VDgRZ",
  stage:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuBoRvSlHsGSK5JYfx8r7praM2C7qfaT8MA3oCiEBrp2qR1Ew_d_BBW1bayhxrA9QACs__BYjSfSKuyEvZcT0YtXO8fuXj8VQ2YLiuTimXER4hQXjdpWsSohnXC6O_Q_Ax3IYrf6kxn3pfnf3gbpdpHg6Z_gBGl-pwwh9QZ1MJMCDFNOgDIYu6YlIUcGa_f9muHACh25ulddKdk1mb9Ml2sMhagIzsTCt5xLaDwtQUM8HjhIkIrThVgRoRpajSVgMilICEgR6TB1uoLn",
  vertical:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuAqJ-TzzKMaUW37h8FSVT6sySUyelD9iqAXQM8_V6Ynq8kYCFHFl-i5bCoymxVX4HRAhhWgD59axTWzgDp0cHyhyxghctwlas-jU5GyIstMv9SFzSLAx6tbBm85-yYz-578vmofewsYO3GeSOn7DOfZehI-h4AYI4TVeLPJp1t2qRfNFfYVTM6wFRmrN6KpTsUf-i1KDnFjGY0jsdTWvNSWT4ESDCXtOBQ9aWp9AzFdF4KNeN2DiNc5TqpFDECYEYYb8xODhOueWS1n",
  creator:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuCIV1PKtZYRoVkb2NMSoMUWK2b4z4ud3anAMawRZcTvvO65A3fdP_VxZ-SKBjbFmKTXq8K4_5u7hDf1HFGvgAeGcRLain2KtgULNWhuvqWY6DarBw00-1b5W5FbUG65hymKyOYSaKWWhutXHzhRpe9P6PtNySTafG8eHDMWiY3Nd98DFbfRucptBxPPwEiuHqa25JIulogR7d149IxiPQBll9Cj5SbLwJJHMJwyYDdjLt6Xeb3SxzEo_wXQRoy_1ygtQV0BrhnwB-Gc",
  elena:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuCfqKNkFePsMLBDZYqyGujFrw7aFgKuMbRmsfxl2RU6YwArmAMkd2WUVUnThLXS1IZ06GUZtBEwByaBo_gZorXeQZDI80CTTRv_UVwE8zOrSzQcYoPs610EFhylJndZzJdHfZ_baQVq05Jrr2nIs7XAaPX61Z2ztTTJW_J15FaeL8r-SltWdmFelv1138YY_ZzeCtPYBsTCVFSJ4jCOoKS346foSAxivJV0V-VKDfC87OD2aNJvmJUGF28t-s9gS2MaTUjbEeLcHAmt",
  marcus:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuB7uaOE97IUWoZ4UATBaoSTyBJZUmaVDIznnBD7dhe-84PxZmlZ1V8lAgzaub9vACzqMryk0r96bzhbWPWr4VFjb6IQKxipytjB0_hO7yATBoAmjoMitTHcKYf-KgpXjA0_I4DP8Kym-JYSOhbsDxtiwXkZk1KHPGMerpuvZm24J8JkfExcRH-_8MFsLcGC6tkPin9XxwLp41Hu7yaTCJ6G2--rRM8JW2W9wtB0kXH3sn6754xv94qNtJNdCnyg7cpmz7tbIePycKd5"
};

const emptyProfile = {
  id: 0,
  account: "",
  nickname: "",
  avatar_url: image.currentUser,
  bio: "",
  role: "",
  status: 0
};

function App() {
  const [route, setRoute] = useState(() => normalizeRoute(window.location.pathname));
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) || "");
  const [user, setUser] = useState(() => readStoredUser());

  useEffect(() => {
    const handlePopState = () => setRoute(normalizeRoute(window.location.pathname));
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  useEffect(() => {
    if (route === "/") {
      navigate("/feed", setRoute);
    }
    if ((route === "/profile" || route === "/me") && !(token && user)) {
      navigate("/feed", setRoute);
    }
  }, [route, token, user]);

  const session = useMemo(
    () => ({
      token,
      user,
      setAuth(nextToken, nextUser) {
        setToken(nextToken);
        setUser(nextUser);
        if (nextToken) {
          localStorage.setItem(TOKEN_KEY, nextToken);
        } else {
          localStorage.removeItem(TOKEN_KEY);
        }
        localStorage.setItem(USER_KEY, JSON.stringify(nextUser));
      },
      clearAuth() {
        setToken("");
        setUser(null);
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(USER_KEY);
      }
    }),
    [token, user]
  );

  if (route === "/auth" || route === "/login") {
    return <LoginPage session={session} onNavigate={(path) => navigate(path, setRoute)} />;
  }

  if (route === "/profile" || route === "/me") {
    return (
      <AppShell
        user={user}
        authenticated={Boolean(token && user)}
        onNavigate={(path) => navigate(path, setRoute)}
        onLogout={() => logout(session, setRoute)}
      >
        <ProfilePage session={session} onNavigate={(path) => navigate(path, setRoute)} />
      </AppShell>
    );
  }

  if (route === "/upload") {
    return (
      <AppShell
        user={user}
        authenticated={Boolean(token && user)}
        onNavigate={(path) => navigate(path, setRoute)}
        onLogout={() => logout(session, setRoute)}
      >
        <UploadPage session={session} onNavigate={(path) => navigate(path, setRoute)} />
      </AppShell>
    );
  }

  return (
    <AppShell
      user={user}
      authenticated={Boolean(token && user)}
      onNavigate={(path) => navigate(path, setRoute)}
      onLogout={() => logout(session, setRoute)}
    >
      <FeedPage session={session} />
    </AppShell>
  );
}

function LoginPage({ session, onNavigate }) {
  const [mode, setMode] = useState("login");
  const [form, setForm] = useState({ account: "", password: "", nickname: "" });
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event) {
    event.preventDefault();
    setSubmitting(true);
    setMessage("");
    try {
      if (mode === "register") {
        await apiRequest("/api/auth/register", {
          method: "POST",
          body: {
            account: form.account.trim(),
            password: form.password,
            nickname: form.nickname.trim()
          }
        });
      }
      const tokenResponse = await apiRequest("/api/auth/login/password", {
        method: "POST",
        body: {
          account: form.account.trim(),
          password: form.password
        }
      });
      const accessToken = tokenResponse.access_token;
      const profile = await apiRequest("/api/users/me", { token: accessToken });
      session.setAuth(accessToken, profile);
      onNavigate("/feed");
    } catch (error) {
      setMessage(error.message || "登录失败，请检查账号与密码");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-visual" aria-label="GCFeed">
        <div className="auth-preview">
          <img src={image.stage} alt="" />
          <div className="auth-preview-card">
            <span className="material-symbols-outlined">play_arrow</span>
            <div>
              <strong>GCFeed</strong>
              <span>16:9 desktop feed</span>
            </div>
          </div>
        </div>
      </section>
      <section className="auth-panel">
        <div className="auth-card">
          <div className="brand-block">
            <span className="brand-mark">GC</span>
            <div>
              <h1>登录 GCFeed</h1>
              <p>连接后端账号、Feed 和个人资料。</p>
            </div>
          </div>

          <form className="auth-form" onSubmit={handleSubmit}>
            <div className="auth-mode-tabs">
              <button className={mode === "login" ? "active" : ""} type="button" onClick={() => setMode("login")}>
                登录
              </button>
              <button className={mode === "register" ? "active" : ""} type="button" onClick={() => setMode("register")}>
                注册
              </button>
            </div>
            <label>
              <span>账号</span>
              <input
                value={form.account}
                onChange={(event) => setForm({ ...form, account: event.target.value })}
                placeholder="account@example.com"
                autoComplete="username"
              />
            </label>
            {mode === "register" && (
              <label>
                <span>昵称</span>
                <input
                  value={form.nickname}
                  onChange={(event) => setForm({ ...form, nickname: event.target.value })}
                  placeholder="输入昵称"
                  autoComplete="nickname"
                />
              </label>
            )}
            <label>
              <span>密码</span>
              <input
                value={form.password}
                onChange={(event) => setForm({ ...form, password: event.target.value })}
                placeholder="输入密码"
                type="password"
                autoComplete="current-password"
              />
            </label>
            {message && <p className="form-message">{message}</p>}
            <button className="primary-button" disabled={submitting}>
              <span className="material-symbols-outlined">login</span>
              {submitting ? "提交中" : mode === "register" ? "注册并登录" : "登录"}
            </button>
          </form>
        </div>
      </section>
    </main>
  );
}

function AppShell({ children, user, authenticated, onNavigate, onLogout }) {
  const displayUser = user || emptyProfile;
  return (
    <div className="app-shell">
      <TopNav user={displayUser} authenticated={authenticated} onNavigate={onNavigate} onLogout={onLogout} />
      <div className="app-body">
        <aside className="sidebar">
          <button className="sidebar-link active" onClick={() => onNavigate("/feed")}>
            <span className="material-symbols-outlined filled">home</span>
            <span>Feed</span>
          </button>
        </aside>
        {children}
      </div>
    </div>
  );
}

function TopNav({ user, authenticated, onNavigate, onLogout }) {
  return (
    <header className="top-nav">
      <div className="top-left">
        <button className="wordmark" onClick={() => onNavigate("/feed")}>
          GCFeed
        </button>
      </div>
      <div className="top-center">
        <label className="search-box">
          <span className="material-symbols-outlined">search</span>
          <input placeholder="Search" />
        </label>
      </div>
      <div className="top-actions">
        <button className="upload-button" onClick={() => onNavigate(authenticated ? "/upload" : "/auth")}>
          <span className="material-symbols-outlined">upload</span>
          Upload
        </button>
        <button className="icon-button" aria-label="Notifications">
          <span className="material-symbols-outlined">notifications</span>
        </button>
        <button
          className={`avatar-button ${authenticated ? "" : "guest"}`}
          onClick={() => onNavigate(authenticated ? "/profile" : "/auth")}
          aria-label={authenticated ? "Profile" : "Login"}
        >
          {authenticated ? (
            <img src={user.avatar_url || image.currentUser} alt="" />
          ) : (
            <>
              <span className="material-symbols-outlined">person</span>
              <span>登录</span>
            </>
          )}
        </button>
        {authenticated && (
          <button className="icon-button" onClick={onLogout} aria-label="Logout">
            <span className="material-symbols-outlined">logout</span>
          </button>
        )}
      </div>
    </header>
  );
}

function FeedPage({ session }) {
  const [items, setItems] = useState([]);
  const [index, setIndex] = useState(0);
  const [liked, setLiked] = useState({});
  const [favorited, setFavorited] = useState({});
  const [commentsOpen, setCommentsOpen] = useState(false);
  const [commentText, setCommentText] = useState("");
  const [feedState, setFeedState] = useState("loading");
  const [feedError, setFeedError] = useState("");
  const wheelLocked = useRef(false);

  const loadFeed = useCallback(() => {
    let live = true;
    setFeedState("loading");
    setFeedError("");
    apiRequest("/api/feed/timeline?limit=12", { token: session.token })
      .then((data) => {
        if (!live) return;
        setItems((data.items || []).map(mapFeedItem));
        setIndex(0);
        setFeedState("ready");
      })
      .catch((error) => {
        if (!live) return;
        setItems([]);
        setFeedError(error.message || "Feed 加载失败");
        setFeedState("error");
      });
    return () => {
      live = false;
    };
  }, [session.token]);

  useEffect(() => {
    return loadFeed();
  }, [loadFeed]);

  const current = items[index];

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (["ArrowDown", "j", "J"].includes(event.key)) {
        setIndex((value) => Math.min(items.length - 1, value + 1));
      }
      if (["ArrowUp", "k", "K"].includes(event.key)) {
        setIndex((value) => Math.max(0, value - 1));
      }
      if (event.key === "l" || event.key === "L") {
        if (!current) return;
        setLiked((state) => ({ ...state, [current.video_id]: !state[current.video_id] }));
      }
      if (event.key === "f" || event.key === "F") {
        if (!current) return;
        setFavorited((state) => ({ ...state, [current.video_id]: !state[current.video_id] }));
      }
      if (event.key === "c" || event.key === "C") {
        setCommentsOpen(true);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [current, items.length]);

  function goNext() {
    setIndex((value) => Math.min(items.length - 1, value + 1));
  }

  function goPrev() {
    setIndex((value) => Math.max(0, value - 1));
  }

  function handleWheel(event) {
    if (Math.abs(event.deltaY) < 32 || wheelLocked.current || items.length < 2) return;
    wheelLocked.current = true;
    if (event.deltaY > 0) {
      goNext();
    } else {
      goPrev();
    }
    window.setTimeout(() => {
      wheelLocked.current = false;
    }, 420);
  }

  return (
    <main className="feed-layout">
      <section className="feed-main" onWheel={handleWheel}>
        {feedState === "loading" && <FeedMessage icon="hourglass_top" title="正在加载 Feed" />}
        {feedState === "error" && (
          <FeedMessage icon="sync_problem" title={feedError} action="重新加载" onAction={loadFeed} />
        )}
        {feedState === "ready" && items.length === 0 && (
          <FeedMessage icon="video_library" title="Feed 暂无视频" action="刷新" onAction={loadFeed} />
        )}
        {current && (
          <div className="feed-stage-wrap">
            <VideoStage
              item={current}
              liked={Boolean(liked[current.video_id])}
              favorited={Boolean(favorited[current.video_id])}
              onLike={() => setLiked((state) => ({ ...state, [current.video_id]: !state[current.video_id] }))}
              onComment={() => setCommentsOpen(true)}
              onFavorite={() =>
                setFavorited((state) => ({ ...state, [current.video_id]: !state[current.video_id] }))
              }
            />
          </div>
        )}
      </section>
      <CommentPanel
        open={commentsOpen}
        value={commentText}
        onChange={setCommentText}
        onClose={() => setCommentsOpen(false)}
        onSubmit={() => setCommentText("")}
        user={session.user || emptyProfile}
        count={current?.comment_count || 0}
      />
    </main>
  );
}

function FeedMessage({ icon, title, action, onAction }) {
  return (
    <div className="feed-message">
      <span className="material-symbols-outlined">{icon}</span>
      <strong>{title}</strong>
      {action && <button onClick={onAction}>{action}</button>}
    </div>
  );
}

function VideoStage({ item, liked, favorited, onLike, onComment, onFavorite }) {
  const cover = item.cover_url || image.stage;
  const media = item.media_url || cover;
  const showVideo = isVideoSource(media);

  return (
    <article className="video-stage">
      <img className="stage-backdrop" src={cover} alt="" />
      <div className="stage-vignette" />
      {showVideo ? (
        <video className="stage-media" src={media} poster={cover} autoPlay muted loop playsInline controls />
      ) : (
        <img className="stage-media portrait-media" src={media} alt="" />
      )}
      <div className="stage-copy">
        <div className="creator-row">
          <img src={item.avatar_url || image.creator} alt="" />
          <div>
            <strong>@{item.author}</strong>
          </div>
          <button>Follow</button>
        </div>
        <h1>{item.title}</h1>
        <p>{item.description}</p>
      </div>
      <div className="action-rail">
        <ActionButton icon="favorite" label={formatMetric(item.like_count + (liked ? 1 : 0))} active={liked} onClick={onLike} />
        <ActionButton icon="chat_bubble" label={formatMetric(item.comment_count)} onClick={onComment} />
        <ActionButton icon="bookmark" label={formatMetric(item.favorite_count + (favorited ? 1 : 0))} active={favorited} onClick={onFavorite} />
        <ActionButton icon="share" label="" compact />
      </div>
      <div className="progress-track">
        <span style={{ width: "34%" }} />
      </div>
    </article>
  );
}

function ActionButton({ icon, label, active, compact, onClick }) {
  return (
    <button className={`rail-button ${active ? "active" : ""} ${compact ? "compact" : ""}`} onClick={onClick}>
      <span className={`material-symbols-outlined ${active ? "filled" : ""}`}>{icon}</span>
      {label && <strong>{label}</strong>}
    </button>
  );
}

function CommentPanel({ open, value, onChange, onSubmit, onClose, user, count }) {
  return (
    <aside className={`comment-panel ${open ? "open" : ""}`}>
      <header className="comment-header">
        <h2>
          Comments <span>{formatMetric(count)}</span>
        </h2>
        <div>
          <button className="icon-button small" aria-label="Filter comments">
            <span className="material-symbols-outlined">tune</span>
          </button>
          <button className="icon-button small" aria-label="Close comments" onClick={onClose}>
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>
      </header>
      <div className="comment-list">
        <div className="comment-empty">
          <span className="material-symbols-outlined">chat_bubble</span>
          <strong>评论接口待接入</strong>
          <p>当前后端已提供 Feed 数据与观看上报，评论列表和发表评论接口接入后会显示真实评论。</p>
        </div>
      </div>
      <form
        className="comment-form"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit();
        }}
      >
        <img src={user.avatar_url || image.currentUser} alt="" />
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="Add a comment..." />
        <button aria-label="Send comment">
          <span className="material-symbols-outlined">send</span>
        </button>
      </form>
    </aside>
  );
}

function ProfilePage({ session, onNavigate }) {
  const baseUser = session.user || emptyProfile;
  const [form, setForm] = useState({
    nickname: baseUser.nickname || "",
    avatar_url: baseUser.avatar_url || "",
    bio: baseUser.bio || ""
  });
  const [avatarFile, setAvatarFile] = useState(null);
  const [avatarPreview, setAvatarPreview] = useState("");
  const [editing, setEditing] = useState(false);
  const [selectedWork, setSelectedWork] = useState(null);
  const [status, setStatus] = useState("");
  const [videos, setVideos] = useState([]);
  const [videosState, setVideosState] = useState("loading");
  const followingCount = baseUser.following_count ?? baseUser.followingCount ?? 0;
  const followerCount = baseUser.follower_count ?? baseUser.followerCount ?? 0;

  useEffect(() => {
    setForm({
      nickname: baseUser.nickname || "",
      avatar_url: baseUser.avatar_url || "",
      bio: baseUser.bio || ""
    });
    setAvatarFile(null);
    setAvatarPreview("");
  }, [baseUser.avatar_url, baseUser.bio, baseUser.nickname]);

  useEffect(() => {
    if (!avatarFile) {
      setAvatarPreview("");
      return;
    }
    const objectURL = URL.createObjectURL(avatarFile);
    setAvatarPreview(objectURL);
    return () => URL.revokeObjectURL(objectURL);
  }, [avatarFile]);

  useEffect(() => {
    if (!session.token) {
      setVideosState("ready");
      return;
    }
    setVideosState("loading");
    apiRequest("/api/videos/mine?limit=12", { token: session.token })
      .then((data) => {
        setVideos(data.items || []);
        setVideosState("ready");
      })
      .catch((error) => {
        if (error.status === 401) {
          session.clearAuth();
          onNavigate("/auth");
          return;
        }
        setVideos([]);
        setVideosState(error.message || "作品加载失败");
      });
  }, [onNavigate, session]);

  async function handleSave(event) {
    event.preventDefault();
    setStatus("保存中");
    try {
      let avatarURL = form.avatar_url;
      if (avatarFile && session.token) {
        const uploaded = await uploadFile(avatarFile, "avatar", session.token);
        avatarURL = uploaded.url;
      }
      let profile = { ...baseUser, ...form };
      if (session.token) {
        profile = await apiRequest("/api/users/me", {
          method: "PATCH",
          token: session.token,
          body: {
            nickname: form.nickname,
            avatar_url: avatarURL,
            bio: form.bio
          }
        });
      }
      session.setAuth(session.token, profile);
      setAvatarFile(null);
      setStatus("已保存");
      setEditing(false);
    } catch (error) {
      setStatus(error.message || "保存失败");
    }
  }

  return (
    <main className="profile-page">
      <section className="profile-hero">
        <div className="profile-summary">
          <img className="profile-avatar" src={avatarPreview || form.avatar_url || image.currentUser} alt="" />
          <div>
            <p className="eyebrow">Creator Profile</p>
            <h1>{form.nickname || baseUser.account}</h1>
            <p>{form.bio || "作品、关注和互动资料会显示在这里。"}</p>
          </div>
          <div className="profile-stats" aria-label="Profile stats">
            <span>
              <strong>{formatMetric(followingCount)}</strong>
              Following
            </span>
            <span>
              <strong>{formatMetric(followerCount)}</strong>
              Followers
            </span>
            <span>
              <strong>{formatMetric(videos.length)}</strong>
              Works
            </span>
          </div>
          <button className="profile-edit-button" onClick={() => setEditing(true)} aria-label="编辑资料">
            <span className="material-symbols-outlined">manage_accounts</span>
          </button>
        </div>
      </section>

      <section className="profile-grid">
        <section className="profile-card works-card">
          <header>
            <h2>我的作品</h2>
            <button className="ghost-button compact" onClick={() => onNavigate("/feed")}>
              <span className="material-symbols-outlined">home</span>
              Feed
            </button>
          </header>
          <div className="work-list">
            {videosState === "loading" && <p className="card-empty">正在加载作品</p>}
            {videosState !== "loading" && typeof videosState === "string" && videosState !== "ready" && (
              <p className="card-empty">{videosState}</p>
            )}
            {videosState === "ready" && videos.length === 0 && <p className="card-empty">暂无作品</p>}
            {videos.map((video) => (
              <button className="work-item" key={video.id || video.video_id} onClick={() => setSelectedWork(video)}>
                <div className="work-thumb">
                  <img src={video.cover_url || image.stage} alt="" />
                  <span className="material-symbols-outlined">play_arrow</span>
                </div>
                <div className="work-meta">
                  <h3>{video.title}</h3>
                  <p>{formatMetric(video.like_count || 0)} likes · {formatMetric(video.comment_count || 0)} comments</p>
                  <span className="status-badge">{video.status === 0 ? "Reviewing" : "Published"}</span>
                </div>
              </button>
            ))}
          </div>
        </section>
      </section>
      {selectedWork && <WorkViewer video={selectedWork} onClose={() => setSelectedWork(null)} />}
      {editing && (
        <div className="modal-backdrop" role="presentation">
          <form className="profile-modal profile-form" onSubmit={handleSave}>
            <header>
              <h2>资料编辑</h2>
              <button className="icon-button small" type="button" onClick={() => setEditing(false)} aria-label="关闭">
                <span className="material-symbols-outlined">close</span>
              </button>
            </header>
            <label>
              <span>昵称</span>
              <input value={form.nickname} onChange={(event) => setForm({ ...form, nickname: event.target.value })} />
            </label>
            <label>
              <span>头像</span>
              <span className="file-picker avatar-picker">
                <span className="avatar-upload-preview">
                  {avatarPreview || form.avatar_url ? (
                    <img src={avatarPreview || form.avatar_url} alt="" />
                  ) : (
                    <span className="material-symbols-outlined">person</span>
                  )}
                </span>
                <span className="file-picker-copy">
                  <strong>{avatarFile ? avatarFile.name : "选择头像文件"}</strong>
                  <small>本地图片上传</small>
                </span>
                <input type="file" accept="image/*" onChange={(event) => setAvatarFile(event.target.files?.[0] || null)} />
              </span>
            </label>
            <label>
              <span>简介</span>
              <textarea value={form.bio} onChange={(event) => setForm({ ...form, bio: event.target.value })} rows={4} />
            </label>
            {status && <p className={`form-message ${status === "已保存" ? "success" : ""}`}>{status}</p>}
            <button className="primary-button">
              <span className="material-symbols-outlined">save</span>
              保存
            </button>
          </form>
        </div>
      )}
    </main>
  );
}

function WorkViewer({ video, onClose }) {
  const media = video.media_url || video.cover_url || image.stage;
  const cover = video.cover_url || image.stage;

  return (
    <div className="modal-backdrop work-viewer-backdrop" role="presentation" onClick={onClose}>
      <section className="work-viewer" onClick={(event) => event.stopPropagation()}>
        <header>
          <div>
            <h2>{video.title || "作品"}</h2>
            <p>{formatMetric(video.like_count || 0)} likes · {formatMetric(video.comment_count || 0)} comments</p>
          </div>
          <button className="icon-button small" type="button" onClick={onClose} aria-label="关闭">
            <span className="material-symbols-outlined">close</span>
          </button>
        </header>
        <div className="work-viewer-stage">
          {isVideoSource(media) ? (
            <video src={media} poster={cover} controls autoPlay playsInline />
          ) : (
            <img src={cover} alt="" />
          )}
        </div>
      </section>
    </div>
  );
}

function UploadPage({ session, onNavigate }) {
  const [form, setForm] = useState({
    title: "",
    description: ""
  });
  const [videoFile, setVideoFile] = useState(null);
  const [coverFile, setCoverFile] = useState(null);
  const [coverPreview, setCoverPreview] = useState("");
  const [status, setStatus] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!coverFile) {
      setCoverPreview("");
      return;
    }
    const objectURL = URL.createObjectURL(coverFile);
    setCoverPreview(objectURL);
    return () => URL.revokeObjectURL(objectURL);
  }, [coverFile]);

  async function handleSubmit(event) {
    event.preventDefault();
    setSubmitting(true);
    setStatus("");
    try {
      if (!videoFile) {
        throw new Error("请选择视频文件");
      }
      if (!coverFile) {
        throw new Error("请选择封面文件");
      }
      const [videoUpload, coverUpload] = await Promise.all([
        uploadFile(videoFile, "video", session.token),
        uploadFile(coverFile, "cover", session.token)
      ]);
      await apiRequest("/api/videos", {
        method: "POST",
        token: session.token,
        headers: {
          "Idempotency-Key": `web-upload-${Date.now()}`
        },
        body: {
          title: form.title.trim(),
          description: form.description.trim(),
          media_url: videoUpload.url,
          cover_url: coverUpload.url
        }
      });
      setStatus("发布成功");
      onNavigate("/profile");
    } catch (error) {
      setStatus(error.message || "发布失败");
    } finally {
      setSubmitting(false);
    }
  }

  if (!session.token) {
    return (
      <main className="upload-page">
        <section className="upload-card">
          <div className="upload-empty">
            <span className="material-symbols-outlined">lock</span>
            <h1>登录后发布视频</h1>
            <button className="primary-button" onClick={() => onNavigate("/auth")}>
              <span className="material-symbols-outlined">login</span>
              登录
            </button>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="upload-page">
      <section className="upload-card">
        <header>
          <div>
            <p className="eyebrow">Upload</p>
            <h1>发布视频</h1>
          </div>
          <button className="ghost-button compact" onClick={() => onNavigate("/feed")}>
            <span className="material-symbols-outlined">home</span>
            Feed
          </button>
        </header>

        <div className="upload-grid">
          <form className="upload-form" onSubmit={handleSubmit}>
            <label>
              <span>标题</span>
              <input
                value={form.title}
                onChange={(event) => setForm({ ...form, title: event.target.value })}
                placeholder="输入视频标题"
              />
            </label>
            <label>
              <span>简介</span>
              <textarea
                value={form.description}
                onChange={(event) => setForm({ ...form, description: event.target.value })}
                placeholder="输入视频简介"
                rows={4}
              />
            </label>
            <label>
              <span>视频</span>
              <span className="file-picker">
                <span className="material-symbols-outlined">movie</span>
                <span className="file-picker-copy">
                  <strong>{videoFile ? videoFile.name : "选择视频文件"}</strong>
                  <small>本地视频上传</small>
                </span>
                <input type="file" accept="video/*" onChange={(event) => setVideoFile(event.target.files?.[0] || null)} />
              </span>
            </label>
            <label>
              <span>封面</span>
              <span className="file-picker">
                <span className="material-symbols-outlined">image</span>
                <span className="file-picker-copy">
                  <strong>{coverFile ? coverFile.name : "选择封面文件"}</strong>
                  <small>本地图片上传</small>
                </span>
                <input type="file" accept="image/*" onChange={(event) => setCoverFile(event.target.files?.[0] || null)} />
              </span>
            </label>
            {status && <p className={`form-message ${status === "发布成功" ? "success" : ""}`}>{status}</p>}
            <button className="primary-button" disabled={submitting}>
              <span className="material-symbols-outlined">publish</span>
              {submitting ? "发布中" : "发布"}
            </button>
          </form>

          <aside className="upload-preview">
            <div className="preview-frame">
              {coverPreview ? <img src={coverPreview} alt="" /> : <span className="material-symbols-outlined">movie</span>}
            </div>
            <div>
              <h2>{form.title || "视频预览"}</h2>
              <p>{form.description || (videoFile ? videoFile.name : "选择本地视频和封面后会提交到后端视频接口。")}</p>
            </div>
          </aside>
        </div>
      </section>
    </main>
  );
}

function normalizeRoute(pathname) {
  if (pathname === "/login") return "/auth";
  if (pathname === "/me") return "/profile";
  if (["/", "/auth", "/feed", "/profile", "/upload"].includes(pathname)) return pathname;
  return "/feed";
}

function navigate(path, setRoute) {
  const nextPath = normalizeRoute(path);
  window.history.pushState({}, "", nextPath);
  setRoute(nextPath);
}

function logout(session, setRoute) {
  session.clearAuth();
  navigate("/feed", setRoute);
}

function readStoredUser() {
  try {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

async function apiRequest(path, options = {}) {
  const headers = {
    Accept: "application/json",
    ...(options.headers || {})
  };
  if (options.body) headers["Content-Type"] = "application/json";
  if (options.token) headers.Authorization = `Bearer ${options.token}`;

  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined
  });

  if (!response.ok) {
    let message = "请求失败";
    try {
      const data = await response.json();
      if (data.error) message = data.error;
      if (data.message) message = data.message;
    } catch {
      message = response.statusText || message;
    }
    const error = new Error(message);
    error.status = response.status;
    throw error;
  }

  if (response.status === 204) return null;
  return response.json();
}

async function uploadFile(file, kind, token) {
  const data = new FormData();
  data.append("file", file);
  data.append("kind", kind);

  const response = await fetch("/api/uploads", {
    method: "POST",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`
    },
    body: data
  });

  if (!response.ok) {
    let message = "上传失败";
    try {
      const payload = await response.json();
      if (payload.error) message = payload.error;
      if (payload.message) message = payload.message;
    } catch {
      message = response.statusText || message;
    }
    throw new Error(message);
  }

  return response.json();
}

function mapFeedItem(item) {
  return {
    video_id: item.video_id,
    author_id: item.author_id,
    title: item.title,
    media_url: item.media_url,
    cover_url: item.cover_url,
    like_count: item.like_count,
    comment_count: item.comment_count,
    favorite_count: item.favorite_count,
    author: item.author_nickname || `creator_${item.author_id}`,
    avatar_url: item.author_avatar_url || image.creator,
    description: item.description || ""
  };
}

function isVideoSource(url) {
  return /\.(mp4|webm|ogg|mov)(\?|#|$)/i.test(url || "");
}

function formatMetric(value) {
  const number = Number(value || 0);
  if (number >= 1000000) return `${(number / 1000000).toFixed(number >= 10000000 ? 0 : 1)}M`;
  if (number >= 1000) return `${(number / 1000).toFixed(number >= 10000 ? 0 : 1)}k`;
  return String(number);
}

export default App;
