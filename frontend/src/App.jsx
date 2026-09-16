import React, { useState, useEffect } from 'react'
import { GridDashboard } from './views/GridDashboard'
import { SingleDisplay } from './views/SingleDisplay'
import './App.css'

export default function App() {
  const [currentRoute, setCurrentRoute] = useState({ view: 'grid', windowNumber: 1 })

  // Parse path or hash on load and on popstate
  useEffect(() => {
    const parseRoute = () => {
      const pathname = window.location.pathname
      const hash = window.location.hash

      // Check /display/:id
      const displayMatch = pathname.match(/^\/display\/(\d+)$/)
      if (displayMatch) {
        setCurrentRoute({ view: 'single', windowNumber: parseInt(displayMatch[1], 10) })
        return
      }

      // Check #display-:id
      const hashMatch = hash.match(/^#display-(\d+)$/)
      if (hashMatch) {
        setCurrentRoute({ view: 'single', windowNumber: parseInt(hashMatch[1], 10) })
        return
      }

      setCurrentRoute({ view: 'grid', windowNumber: 1 })
    }

    parseRoute()
    window.addEventListener('popstate', parseRoute)
    window.addEventListener('hashchange', parseRoute)

    return () => {
      window.removeEventListener('popstate', parseRoute)
      window.removeEventListener('hashchange', parseRoute)
    }
  }, [])

  const navigateToDisplay = (windowNumber) => {
    window.location.hash = `#display-${windowNumber}`
    setCurrentRoute({ view: 'single', windowNumber })
  }

  const navigateToGrid = () => {
    window.location.hash = ''
    setCurrentRoute({ view: 'grid', windowNumber: 1 })
  }

  return (
    <div className="app-root">
      {currentRoute.view === 'single' ? (
        <SingleDisplay
          windowNumber={currentRoute.windowNumber}
          onBack={navigateToGrid}
        />
      ) : (
        <GridDashboard onNavigateDisplay={navigateToDisplay} />
      )}
    </div>
  )
}
