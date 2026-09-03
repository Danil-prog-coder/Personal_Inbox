import { useCallback, useEffect, useState } from 'react';
import { Rail, TabBar, TABS } from './components/Navigation';
import type { Tab } from './components/Navigation';
import { Notice } from './components/Notice';
import { Toast } from './components/Toast';
import { useIsDesktop } from './hooks/useMediaQuery';
import { useStream } from './hooks/useStream';
import { OnboardingScreen } from './screens/OnboardingScreen';
import { ConnectionsScreen } from './screens/ConnectionsScreen';
import { FeedScreen } from './screens/FeedScreen';
import { SettingsScreen } from './screens/SettingsScreen';
import { SummaryScreen } from './screens/SummaryScreen';
import { api } from './lib/api';
import type { Connection, Density, SourceCard, SourceKind, Theme, User } from './lib/types';

const THEME_KEY = 'pi-theme';
const DENSITY_KEY = 'pi-density';
// Онбординг спрашивают один раз на установку. Отметка локальная: на сервере
// для неё нет поля, а пустые критерии — законный выбор («Пропустить»),
// по ним отличить «ещё не спрашивали» нельзя (решение №50).
const ONBOARDED_KEY = 'pi-onboarded';

function readStored<T extends string>(key: string, fallback: T): T {
  try {
    return (localStorage.getItem(key) as T) || fallback;
  } catch {
    return fallback;
  }
}

export function App() {
  // undefined — профиль ещё грузится, null — до сервера не достучались.
  // Третьего состояния нет: входа в приложение больше не существует.
  const [user, setUser] = useState<User | null | undefined>(undefined);
  const [onboarding, setOnboarding] = useState(false);
  const [tab, setTab] = useState<Tab>('feed');
  const [openedSource, setOpenedSource] = useState<SourceKind | null>(null);
  const [openMessageId, setOpenMessageId] = useState<number | null>(null);
  const [sources, setSources] = useState<SourceCard[]>([]);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [toast, setToast] = useState('');
  const [theme, setTheme] = useState<Theme>(() => readStored<Theme>(THEME_KEY, 'dark'));
  const [density, setDensity] = useState<Density>(() =>
    readStored<Density>(DENSITY_KEY, 'spacious'),
  );

  const isDesktop = useIsDesktop();

  // Тема и плотность живут на корневом элементе: от них зависят все токены.
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.dataset.density = density;
    try {
      localStorage.setItem(THEME_KEY, theme);
      localStorage.setItem(DENSITY_KEY, density);
    } catch {
      // Приватный режим — просто не запоминаем выбор.
    }
  }, [theme, density]);

  const loadData = useCallback(async () => {
    const [nextSources, nextConnections] = await Promise.all([
      api.sources(),
      api.connections(),
    ]);
    setSources(nextSources);
    setConnections(nextConnections);
  }, []);

  const loadProfile = useCallback(() => {
    setUser(undefined);
    api
      .me()
      .then((profile) => {
        setUser(profile);
        setTheme(profile.theme);
        setDensity(profile.density);
        // Критерии пустые и об этом ещё не спрашивали — значит установка новая.
        setOnboarding(!profile.criteria && !readStored(ONBOARDED_KEY, ''));
      })
      .catch(() => setUser(null));
  }, []);

  useEffect(loadProfile, [loadProfile]);

  useEffect(() => {
    if (user) void loadData();
  }, [user, loadData]);

  const streamEvent = useStream(Boolean(user), () => {
    void loadData();
  });

  // Любое событие меняет счётчики на карточках источников.
  useEffect(() => {
    if (streamEvent) void loadData();
  }, [streamEvent, loadData]);

  const showToast = useCallback((text: string) => setToast(text), []);
  const clearOpenMessage = useCallback(() => setOpenMessageId(null), []);

  const applyTheme = (next: Theme) => {
    setTheme(next);
    void api.updateMe({ theme: next });
  };

  const applyDensity = (next: Density) => {
    setDensity(next);
    void api.updateMe({ density: next });
  };

  const openMessageFromSummary = async (id: number) => {
    const message = await api.message(id);
    setTab('feed');
    setOpenedSource(message.source);
    setOpenMessageId(id);
  };

  if (user === undefined) {
    return (
      <>
        <div className="page-bg" />
        <div className="app" />
      </>
    );
  }

  // Сервер не ответил. Раньше на этом месте был экран входа, и он же случайно
  // работал заглушкой: теперь причину надо назвать прямо.
  if (user === null) {
    return (
      <Notice
        title="Нет связи с сервером"
        text="Приложение не смогло получить данные. Проверьте, запущен ли бэкенд."
        actionLabel="Повторить"
        onAction={loadProfile}
      />
    );
  }

  if (onboarding) {
    return (
      <>
        <div className="page-bg" />
        <div className="app">
          <OnboardingScreen
            onDone={async (criteria) => {
              if (criteria.trim()) {
                const result = await api.updateMe({ criteria });
                setUser(result.user);
              }
              try {
                localStorage.setItem(ONBOARDED_KEY, 'yes');
              } catch {
                // Приватный режим — спросим ещё раз в следующий запуск.
              }
              setOnboarding(false);
            }}
          />
        </div>
      </>
    );
  }

  const unread = sources.reduce((sum, card) => sum + card.unread, 0);
  const reauth = connections.filter((item) => item.state === 'reauth').length;

  return (
    <>
      <div className="page-bg" />
      <div className="app">
        {isDesktop && (
          <Rail
            active={tab}
            unread={unread}
            reauth={reauth}
            onNavigate={(next) => {
              setTab(next);
              if (next !== 'feed') setOpenedSource(null);
            }}
          />
        )}

        {tab === 'feed' && (
          <FeedScreen
            sources={sources}
            openedSource={openedSource}
            onOpenSource={setOpenedSource}
            streamEvent={streamEvent}
            onDataChanged={loadData}
            openMessageId={openMessageId}
            onMessageOpened={clearOpenMessage}
          />
        )}
        {tab === 'summary' && <SummaryScreen onOpenMessage={openMessageFromSummary} />}
        {tab === 'connections' && (
          <ConnectionsScreen
            connections={connections}
            onChanged={loadData}
            onToast={showToast}
          />
        )}
        {tab === 'settings' && (
          <SettingsScreen
            user={user}
            theme={theme}
            density={density}
            onThemeChange={applyTheme}
            onDensityChange={applyDensity}
            onUserChange={setUser}
            onGoConnections={() => setTab('connections')}
            onToast={showToast}
          />
        )}

        {!isDesktop && (
          <TabBar
            active={tab}
            onNavigate={(next) => {
              setTab(next);
              if (next !== 'feed') setOpenedSource(null);
            }}
          />
        )}

        {toast && <Toast text={toast} onHide={() => setToast('')} />}
      </div>
    </>
  );
}

export { TABS };
