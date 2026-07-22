import { Route, Routes } from 'react-router-dom'
import { Shell } from './components/Shell'
import { Fleet } from './pages/Fleet'
import { Board } from './pages/Board'
import { Ticket } from './pages/Ticket'
import { Agents } from './pages/Agents'

export function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route path="/" element={<Fleet />} />
        <Route path="/p/:id" element={<Board />} />
        <Route path="/p/:id/t/:tid" element={<Ticket />} />
        <Route path="/agents" element={<Agents />} />
      </Route>
    </Routes>
  )
}
