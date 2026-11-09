import { useState } from 'react'
import PhoneFrame from './components/PhoneFrame'
import RegistrationScreen from './screens/RegistrationScreen'
import MainScreen from './screens/MainScreen'
import ProfileScreen from './screens/ProfileScreen'
import PollScreen from './screens/PollScreen'
import LoaderScreen from './screens/LoaderScreen'
import ResultScreen from './screens/ResultScreen'

export type Screen = 'registration' | 'main' | 'profile' | 'poll' | 'loader' | 'results'

export interface AppData {
  holiday: string
  budget: string
  preferences: string
}

export default function App() {
  const [screen, setScreen] = useState<Screen>('registration')
  const [formData, setFormData] = useState<AppData>({ holiday: '', budget: '', preferences: '' })

  const go = (s: Screen) => setScreen(s)

  return (
    <div className="min-h-screen bg-canvas flex items-center justify-center" style={{ fontFamily: 'var(--font-body)' }}>
      {/* Ambient glows */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden">
        <div
          className="absolute rounded-full"
          style={{
            top: '20%', left: '50%', transform: 'translate(-50%, -50%)',
            width: 600, height: 600,
            background: 'radial-gradient(circle, rgba(212,119,138,0.12) 0%, transparent 70%)',
          }}
        />
        <div
          className="absolute rounded-full"
          style={{
            bottom: '15%', right: '25%',
            width: 400, height: 400,
            background: 'radial-gradient(circle, rgba(232,160,122,0.08) 0%, transparent 70%)',
          }}
        />
      </div>

      {/* Background label */}
      <div className="fixed bottom-8 left-1/2 -translate-x-1/2 text-center pointer-events-none select-none">
        <p style={{ fontFamily: 'var(--font-display)', color: 'rgba(245,235,232,0.04)', fontSize: 64, letterSpacing: '0.2em' }}>
          Amore
        </p>
      </div>

      <PhoneFrame>
        {screen === 'registration' && <RegistrationScreen onNext={() => go('main')} />}
        {screen === 'main' && (
          <MainScreen
            onGenerate={(data) => { setFormData(data); go('loader') }}
            onProfile={() => go('profile')}
          />
        )}
        {screen === 'profile' && (
          <ProfileScreen
            onGeneratePoll={() => go('poll')}
            onSignOut={() => go('registration')}
            onBack={() => go('main')}
          />
        )}
        {screen === 'poll' && <PollScreen onComplete={() => go('main')} onBack={() => go('profile')} />}
        {screen === 'loader' && <LoaderScreen onComplete={() => go('results')} data={formData} />}
        {screen === 'results' && <ResultScreen onBack={() => go('main')} data={formData} />}
      </PhoneFrame>
    </div>
  )
}
