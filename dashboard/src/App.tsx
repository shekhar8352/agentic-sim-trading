import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { AgentDetailPage } from './pages/AgentDetailPage'
import { ComparePage } from './pages/ComparePage'
import { SimulationLivePage } from './pages/SimulationLivePage'
import { SimulationsPage } from './pages/SimulationsPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/simulations" replace />} />
          <Route path="/simulations" element={<SimulationsPage />} />
          <Route path="/simulation/:id" element={<SimulationLivePage />} />
          <Route path="/agent/:id" element={<AgentDetailPage />} />
          <Route path="/compare" element={<ComparePage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
