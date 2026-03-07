import { useState, useEffect } from 'react';
import { MapContainer, TileLayer, CircleMarker, useMapEvents, Popup } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';

type Message = {
  id: number;
  sender: 'user' | 'agent';
  text: string;
};

type MapData = {
  lat: number;
  lng: number;
  score: number;
  name: string;
  type: 'competitor' | 'opportunity';
};

type LocationEval = {
  lat: number;
  lng: number;
  footTraffic: string;
  nearbyCompetitors: number;
  opportunityScore: number;
  demographicProfile: string;
  estCosts: string;
  reviewCount: number;
  statsExtra: string;
  calcLog: string;
  citywideActiveTaxCompetitor: number;
};

// Component to track map movement and update bounds for backend query
function MapEventHandler({ onBoundsChange, onLocationSelect }: { 
  onBoundsChange: (n: number, s: number, e: number, w: number) => void,
  onLocationSelect: (lat: number, lng: number) => void 
}) {
  useMapEvents({
    moveend: (e) => {
      const bounds = e.target.getBounds();
      onBoundsChange(bounds.getNorth(), bounds.getSouth(), bounds.getEast(), bounds.getWest());
    },
    click: (e) => {
      onLocationSelect(e.latlng.lat, e.latlng.lng);
    }
  });
  return null;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<'home' | 'map' | 'db_explorer'>('map');
  const[activeWorkspace, setActiveWorkspace] = useState('General Food Business');
  const [messages, setMessages] = useState<Message[]>([
    { id: 1, sender: 'agent', text: 'Hello. I am the Nourish PT Data Agent backed by the AI Gateway. How can I help you find gaps in the market today?' }
  ]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [activeLayers, setActiveLayers] = useState<string[]>(['base_map']);
  
  const [naicsFilter, setNaicsFilter] = useState('445');
  const[foodDesertFilter, setFoodDesertFilter] = useState('all');
  const [rentFilter, setRentFilter] = useState('any');
  const [popularityFilter, setPopularityFilter] = useState('high');
  const[mapPoints, setMapPoints] = useState<MapData[]>([]);
  const[debugInfo, setDebugInfo] = useState<any>(null);

  // Manual Filter Toggles for Scoring Math
  const[weightFootTraffic, setWeightFootTraffic] = useState(true);
  const[weightCompetitors, setWeightCompetitors] = useState(true);
  const [weightCosts, setWeightCosts] = useState(true);

  // Settings State for UI-based manual API Key overrides
  const [showAgentSettings, setShowAgentSettings] = useState(false);
  const [llmProvider, setLlmProvider] = useState(localStorage.getItem('llm_provider') || 'NRP');
  const [llmApiKey, setLlmApiKey] = useState(localStorage.getItem('llm_api_key') || '');
  const[llmModel, setLlmModel] = useState(localStorage.getItem('llm_model') || 'gpt-oss');
  const [llmBaseUrl, setLlmBaseUrl] = useState(localStorage.getItem('llm_base_url') || '');

  useEffect(() => {
    localStorage.setItem('llm_provider', llmProvider);
    localStorage.setItem('llm_api_key', llmApiKey);
    localStorage.setItem('llm_model', llmModel);
    localStorage.setItem('llm_base_url', llmBaseUrl);
  },[llmProvider, llmApiKey, llmModel, llmBaseUrl]);

  // Toggle state to flip between strict Points view and Gradient Heatmap view
  const[heatmapMode, setHeatmapMode] = useState(true);

  const [mapBounds, setMapBounds] = useState<{n: number, s: number, e: number, w: number} | null>({
    n: 32.95, s: 32.65, e: -116.95, w: -117.30
  });
  
  const[selectedLocation, setSelectedLocation] = useState<{lat: number, lng: number} | null>(null);
  const[locationEval, setLocationEval] = useState<LocationEval | null>(null);
  const[isEvaluating, setIsEvaluating] = useState(false);

  const [exploreTable, setExploreTable] = useState('nourish_cbg_food_environment');
  const [exploreResult, setExploreResult] = useState('');

  const quickTables =[
    "nourish_cbg_food_environment",
    "nourish_cbg_pedestrian_flow",
    "san_diego_areawise_foot_traffic",
    "ca_laws_and_regulations",
    "nourish_cbg_demographics",
    "esri_consumer_spending_data_",
    "sandag_layer_zoning_base_sd_new",
    "nourish_comm_commissary_ext",
    "nourish_ref_mobile_vendor_economics"
  ];

  const handleLocationSelect = async (lat: number, lng: number) => {
    setSelectedLocation({ lat, lng });
    setIsEvaluating(true);
    setLocationEval(null);

    try {
      const response = await fetch(`http://localhost:8081/api/evaluate-location?lat=${lat}&lng=${lng}&naics=${naicsFilter}&useFootTraffic=${weightFootTraffic}&useCompetitors=${weightCompetitors}&useCosts=${weightCosts}`);
      const data = await response.json();
      setLocationEval(data);
    } catch (error) {
      console.error("Evaluation failed", error);
    } finally {
      setIsEvaluating(false);
    }
  };

  const handleSendMessage = async () => {
    if (!inputValue.trim()) return;

    const userMsg: Message = { id: Date.now(), sender: 'user', text: inputValue };
    setMessages((prev) => [...prev, userMsg]);
    setInputValue('');
    setIsLoading(true);

    try {
      const response = await fetch('http://localhost:8081/api/agent/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          message: userMsg.text,
          apiKey: llmApiKey,
          model: llmModel,
          provider: llmProvider,
          baseUrl: llmBaseUrl
        }),
      });
      const data = await response.json();
      
      setMessages((prev) =>[...prev, { id: Date.now(), sender: 'agent', text: data.reply }]);
      if (data.activeLayers) setActiveLayers(data.activeLayers);
      if (data.mapPoints) setMapPoints(data.mapPoints ||[]);
      if (data.activeWorkspace) setActiveWorkspace(data.activeWorkspace);
      
    } catch (error) {
      setMessages((prev) =>[...prev, { id: Date.now(), sender: 'agent', text: 'Error connecting to the backend LLM.' }]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleManualSearch = async () => {
    setIsLoading(true);
    try {
      let url = `http://localhost:8081/api/opportunity-map?naics=${naicsFilter}&foodDesert=${foodDesertFilter}&rent=${rentFilter}&popularity=${popularityFilter}&useFootTraffic=${weightFootTraffic}&useCompetitors=${weightCompetitors}&useCosts=${weightCosts}`;
      if (mapBounds) {
        url += `&n=${mapBounds.n}&s=${mapBounds.s}&e=${mapBounds.e}&w=${mapBounds.w}`;
      }
      
      const response = await fetch(url);
      const payload = await response.json();
      setMapPoints(payload.data?.points ||[]);
      setDebugInfo(payload.data?.debug || null);
      setActiveLayers([
        `NAICS Prefix: ${naicsFilter}`, 
        `Food Desert Profile: ${foodDesertFilter}`
      ]);
    } catch (error) {
      console.error("Manual search failed", error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleMapChange = (n: number, s: number, e: number, w: number) => {
    setMapBounds({ n, s, e, w });
  };

  useEffect(() => {
    if (activeTab === 'map' && mapBounds) {
      handleManualSearch();
    }
  },[naicsFilter, foodDesertFilter, rentFilter, popularityFilter, weightFootTraffic, weightCompetitors, weightCosts, mapBounds, activeTab]);

  const handleExploreDB = async () => {
    setExploreResult('Querying database...');
    try {
      const response = await fetch(`http://localhost:8081/api/explore-db?table=${exploreTable}`);
      const data = await response.json();
      setExploreResult(JSON.stringify(data, null, 2));
    } catch (error) {
      setExploreResult('Error connecting to DB or table does not exist.\nEnsure the backend has successfully connected to SDSC server.');
    }
  };

  const renderMessage = (text: string) => {
    const parts = text.split('```');
    return parts.map((part, index) => {
      if (index % 2 === 1) {
        const lines = part.split('\n');
        const lang = lines[0];
        const code = lines.slice(1).join('\n');
        return (
          <pre key={index} style={{ backgroundColor: '#f0f4f9', color: '#1f1f1f', padding: '12px', borderRadius: '8px', overflowX: 'auto', marginTop: '8px', marginBottom: '8px', fontSize: '13px', fontFamily: 'monospace', border: '1px solid #e0e0e0' }}>
            <code>{code || lang}</code>
          </pre>
        );
      }
      
      let html = part.replace(/\n/g, '<br/>').replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
      return <span key={index} dangerouslySetInnerHTML={{__html: html}} />;
    });
  };

  const getHeatmapColor = (score: number) => {
    if (score > 85) return '#b2182b'; // Dark red/hot
    if (score > 70) return '#ef8a62'; // Orange
    if (score > 50) return '#fddbc7'; // Light orange
    if (score > 35) return '#d1e5f0'; // Cool
    return '#92c5de'; // Coldest
  };

  const opportunityPoints = mapPoints.filter(p => p.type === 'opportunity');
  const maxScore = opportunityPoints.length > 0 ? Math.max(...opportunityPoints.map(p => p.score)) : 0;

  return (
    <>
      <header className="cloud-header">
        <div className="cloud-header-logo">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="#0b57d0">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z"/>
          </svg>
          Nourish PT
        </div>
        <div className="cloud-header-project">Live Food Business Opportunity Mapper</div>
      </header>

      <div className="app-container">
        <aside className="sidebar">
          <div className="sidebar-header">Application Views</div>
          <div className={`sidebar-item ${activeTab === 'home' ? 'active' : ''}`} onClick={() => setActiveTab('home')}>Methodology & Home</div>
          <div className={`sidebar-item ${activeTab === 'map' ? 'active' : ''}`} onClick={() => setActiveTab('map')}>Opportunity Map</div>
          <div className={`sidebar-item ${activeTab === 'db_explorer' ? 'active' : ''}`} onClick={() => setActiveTab('db_explorer')}>Database Explorer</div>
          <div className="sidebar-header" style={{marginTop: '16px'}}>Active Data Sources</div>
          <div className="sidebar-item">ca_business (SD Tax + GM)</div>
          <div className="sidebar-item">nourish_cbg_pedestrian_flow</div>
          <div className="sidebar-item">nourish_cbg_food_environment</div>
          <div className="sidebar-item">sandag_layer_zoning</div>
        </aside>

        <main className="main-workspace">
          <div className="workspace-header">
            <h1 className="workspace-title">
              {activeTab === 'home' && 'Methodology & Application Guide'}
              {activeTab === 'map' && `Opportunity Map | Active Context: ${activeWorkspace}`}
              {activeTab === 'db_explorer' && 'Database Schema Explorer'}
            </h1>
          </div>
          
          <div className="workspace-body">
            {activeTab === 'home' && (
              <div className="home-container">
                <div className="home-card">
                  <h2>Welcome to the Nourish PT Platform</h2>
                  <p>This application helps identify the optimal streets and block groups in San Diego County to establish new food businesses. It queries live data directly from the PostgreSQL data warehouse to find market gaps.</p>
                  
                  <h3>The Opportunity Scoring Methodology</h3>
                  <div className="equation-box">
                    Opportunity Score = Base (45) + Foot Traffic Impact - Cost Penalties - Competition Penalties + Market Gap Bonus
                  </div>

                  <h3>Data Sources Being Queried</h3>
                  <ul>
                    <li><strong>ca_business:</strong> Pinpoints competitor locations (augmented with Google Maps review counts & ratings).</li>
                    <li><strong>nourish_cbg_pedestrian_flow & san_diego_areawise_foot_traffic:</strong> UCSF foot traffic data to estimate organic walk-in volume.</li>
                    <li><strong>nourish_cbg_food_environment:</strong> USDA food desert block group cross-referencing.</li>
                    <li><strong>sandag_layer_zoning_base_sd_new:</strong> Commercial and Mixed-Use development zones.</li>
                  </ul>
                </div>
              </div>
            )}

            {activeTab === 'map' && (
              <>
                <div className="manual-panel">
                  <h2 className="panel-title">Scoring Algorithm Weighting</h2>
                  
                  <div className="control-group">
                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                      <input type="checkbox" checked={weightFootTraffic} onChange={e => setWeightFootTraffic(e.target.checked)} />
                      Consider Foot Traffic / Popularity
                    </label>
                  </div>
                  <div className="control-group">
                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                      <input type="checkbox" checked={weightCompetitors} onChange={e => setWeightCompetitors(e.target.checked)} />
                      Consider Competitor Density (Penalize)
                    </label>
                  </div>
                  <div className="control-group">
                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                      <input type="checkbox" checked={weightCosts} onChange={e => setWeightCosts(e.target.checked)} />
                      Consider Area Land / Operating Costs
                    </label>
                  </div>

                  <hr style={{margin: '16px 0', borderColor: '#e0e0e0'}} />

                  <h2 className="panel-title">Data Filter Configuration</h2>
                  
                  <div className="control-group">
                    <label>View Mode</label>
                    <div style={{ display: 'flex', gap: '8px' }}>
                      <button 
                        className={`primary-btn ${heatmapMode ? '' : 'inactive-btn'}`} 
                        style={{ padding: '6px 12px', flex: 1, backgroundColor: heatmapMode ? '#0b57d0' : '#e0e0e0', color: heatmapMode ? 'white' : '#444' }} 
                        onClick={() => setHeatmapMode(true)}
                      >
                        Gradient Heatmap
                      </button>
                      <button 
                        className={`primary-btn ${!heatmapMode ? '' : 'inactive-btn'}`} 
                        style={{ padding: '6px 12px', flex: 1, backgroundColor: !heatmapMode ? '#0b57d0' : '#e0e0e0', color: !heatmapMode ? 'white' : '#444' }} 
                        onClick={() => setHeatmapMode(false)}
                      >
                        Strict Points
                      </button>
                    </div>
                  </div>

                  <div className="control-group">
                    <label>Business Vertical (NAICS Base)</label>
                    <select className="control-select" value={naicsFilter} onChange={e => setNaicsFilter(e.target.value)}>
                      <option value="445">Food and Beverage Stores (445)</option>
                      <option value="722">Food Services and Drinking (722)</option>
                      <option value="454">Nonstore Retailers / Food Trucks (454)</option>
                    </select>
                  </div>

                  <div className="control-group">
                    <label>Food Access / Desert Focus</label>
                    <select className="control-select" value={foodDesertFilter} onChange={e => setFoodDesertFilter(e.target.value)}>
                      <option value="usda_official">USDA Official Food Deserts</option>
                      <option value="low_income">Low Income / Low Access</option>
                      <option value="all">All Areas</option>
                    </select>
                  </div>

                  <div className="control-group">
                    <label>Cost of Land / Rent Target</label>
                    <select className="control-select" value={rentFilter} onChange={e => setRentFilter(e.target.value)}>
                      <option value="under_30">Under $30 / sqft (Affordable)</option>
                      <option value="under_50">Under $50 / sqft</option>
                      <option value="any">Any Price</option>
                    </select>
                  </div>

                  <div className="control-group">
                    <label>Pedestrian Popularity Target</label>
                    <select className="control-select" value={popularityFilter} onChange={e => setPopularityFilter(e.target.value)}>
                      <option value="high">High Pedestrian Flow Only</option>
                      <option value="medium">Medium Flow +</option>
                      <option value="any">Any Popularity Level</option>
                    </select>
                  </div>

                  <hr style={{margin: '24px 0', borderColor: '#e0e0e0'}} />
                  <h2 className="panel-title" style={{marginBottom: '16px'}}>⚙️ Calculation Logs</h2>
                  {debugInfo ? (
                    <div style={{fontSize: '13px', color: '#444746', lineHeight: '1.6'}}>
                      <div><strong>DB Status:</strong> {debugInfo.dbStatus}</div>
                      <div><strong>SQL Points (Zoning):</strong> {debugInfo.sqlPointsFound}</div>
                      <div><strong>CSV Fallback (GM):</strong> {debugInfo.csvFallbackFound}</div>
                      <div><strong>Competitors Found:</strong> {debugInfo.competitorsFound}</div>
                      <div><strong>Total Map Nodes:</strong> {debugInfo.totalPoints}</div>
                    </div>
                  ) : (
                    <div style={{fontSize: '13px', color: '#747775'}}>Waiting for map sync...</div>
                  )}
                </div>

                <div className="map-container">
                  <div className="map-overlay">
                    <strong>Highest Score Highlighted: {maxScore > 0 ? maxScore : '...'}</strong>
                    {activeLayers.map((layer, i) => <div key={i} style={{color: '#0b57d0', fontSize: '13px', marginTop: '4px'}}>{layer}</div>)}
                    <div style={{marginTop: '8px', color: '#444746'}}>
                      {heatmapMode ? 'Showing Canvas-Rendered Gradient Heatmap.' : 'Showing Precise Plot Marker Points.'}<br/>
                      <em>Tip: Click any marker to view real data & economics.</em>
                    </div>
                  </div>

                  {selectedLocation && (
                    <div className="evaluation-panel">
                      <h3>📍 Location Evaluation</h3>
                      {isEvaluating ? (
                        <p style={{fontSize: '14px', color: '#444746'}}>Running database queries on coordinates...</p>
                      ) : locationEval ? (
                        <>
                          <div className="eval-metric">
                            <span>Area Foot Traffic:</span>
                            <span>{locationEval.footTraffic}</span>
                          </div>
                          <div className="eval-metric">
                            <span>GM Competitors (~2mi):</span>
                            <span>{locationEval.nearbyCompetitors}</span>
                          </div>
                          <div className="eval-metric">
                            <span>Neighborhood Context:</span>
                            <span>{locationEval.demographicProfile}</span>
                          </div>
                          <div className="eval-metric">
                            <span>Est. Operational Costs:</span>
                            <span>{locationEval.estCosts}</span>
                          </div>
                          <div className="eval-metric">
                            <span>Area Context Proxy:</span>
                            <span style={{textAlign: 'right'}}>{locationEval.statsExtra}</span>
                          </div>
                          
                          <div style={{ marginTop: '12px', padding: '8px', backgroundColor: '#f0f4f9', borderRadius: '8px', fontSize: '12px', fontFamily: 'monospace', color: '#041e49' }}>
                            <strong>Calc Trace:</strong><br/>
                            {locationEval.calcLog}
                          </div>

                          <div className="eval-metric" style={{marginTop: '16px', border: 'none', fontSize: '16px'}}>
                            <span><strong>Opp. Score (0-100):</strong></span>
                            <span style={{color: locationEval.opportunityScore > 70 ? '#0f9d58' : locationEval.opportunityScore < 30 ? '#444746' : '#db4437'}}>
                              {locationEval.opportunityScore}
                            </span>
                          </div>
                        </>
                      ) : null}
                    </div>
                  )}

                  <MapContainer 
                    center={[32.847, -117.273]} 
                    zoom={12} 
                    style={{ height: '100%', width: '100%', minHeight: '600px' }}
                    preferCanvas={true}
                  >
                    <TileLayer
                      url="https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png"
                      attribution='&copy; OpenStreetMap &copy; CARTO'
                    />
                    
                    <MapEventHandler onBoundsChange={handleMapChange} onLocationSelect={handleLocationSelect} />

                    {mapPoints.filter(p => p.type === 'competitor').map((p, i) => (
                      <CircleMarker 
                        key={`comp-${i}`} 
                        center={[p.lat, p.lng]} 
                        radius={heatmapMode ? 4 : 3} 
                        pathOptions={{ color: '#ffffff', weight: 1, fillColor: '#000000', fillOpacity: 0.5 }}
                        eventHandlers={{ click: () => handleLocationSelect(p.lat, p.lng) }}
                      >
                        <Popup>{p.name}</Popup>
                      </CircleMarker>
                    ))}

                    {mapPoints.filter(p => p.type === 'opportunity').map((p, i) => {
                      const isTopScore = p.score === maxScore && maxScore > 50;
                      return (
                        <CircleMarker 
                          key={`opp-${i}`} 
                          center={[p.lat, p.lng]} 
                          radius={heatmapMode ? (isTopScore ? 35 : 15 + (p.score / 5)) : (isTopScore ? 10 : 5)} 
                          pathOptions={
                            heatmapMode 
                              ? { stroke: isTopScore, color: isTopScore ? '#fbbc04' : undefined, weight: 3, fillColor: getHeatmapColor(p.score), fillOpacity: p.score < 30 ? 0.2 : 0.6 }
                              : { stroke: true, color: isTopScore ? '#fbbc04' : '#1f1f1f', weight: isTopScore ? 3 : 1, fillColor: getHeatmapColor(p.score), fillOpacity: 0.9 }
                          }
                          eventHandlers={{ click: () => handleLocationSelect(p.lat, p.lng) }}
                        >
                           <Popup>
                             <strong>{p.name} {isTopScore && "🌟 Highest Score"}</strong><br/>
                             Opportunity Score: {p.score}
                           </Popup>
                        </CircleMarker>
                      );
                    })}

                    {selectedLocation && (
                      <CircleMarker 
                        center={[selectedLocation.lat, selectedLocation.lng]} 
                        radius={8} 
                        pathOptions={{ color: '#000000', weight: 2, fillColor: '#ffffff', fillOpacity: 1 }}
                      />
                    )}
                  </MapContainer>
                </div>

                <div className="agent-panel">
                  <div className="agent-header">
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flex: 1 }}>
                      <div className="agent-icon">✨</div>
                      A2A Data Agent
                    </div>
                    <button 
                      onClick={() => setShowAgentSettings(!showAgentSettings)} 
                      style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '18px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                      title="Agent LLM Settings"
                    >
                      ⚙️
                    </button>
                  </div>
                  
                  {showAgentSettings && (
                    <div style={{ padding: '16px', background: '#f8f9fa', borderBottom: '1px solid #e0e0e0', zIndex: 10 }}>
                      <h4 style={{ marginBottom: '12px', fontSize: '14px', color: '#1f1f1f' }}>LLM Configuration</h4>
                      
                      <div className="control-group" style={{ marginBottom: '12px' }}>
                        <label>AI Provider</label>
                        <select className="control-select" value={llmProvider} onChange={e => setLlmProvider(e.target.value)}>
                          <option value="NRP">NRP AI Gateway (OpenAI Spec)</option>
                          <option value="OpenAI">Custom OpenAI Endpoint</option>
                          <option value="Gemini">Google Gemini (AI Studio)</option>
                        </select>
                      </div>

                      <div className="control-group" style={{ marginBottom: '12px' }}>
                        <label>API Key / Bearer Token</label>
                        <input 
                          type="password" 
                          value={llmApiKey} 
                          onChange={e => setLlmApiKey(e.target.value)} 
                          className="control-input" 
                          placeholder={llmProvider === 'Gemini' ? "AIzaSy..." : "ey..."} 
                        />
                      </div>
                      
                      {llmProvider === 'OpenAI' && (
                        <div className="control-group" style={{ marginBottom: '12px' }}>
                          <label>Base URL Override</label>
                          <input 
                            type="text" 
                            value={llmBaseUrl} 
                            onChange={e => setLlmBaseUrl(e.target.value)} 
                            className="control-input" 
                            placeholder="https://api.openai.com/v1/chat/completions" 
                          />
                        </div>
                      )}

                      <div className="control-group" style={{ marginBottom: '16px' }}>
                        <label>Model Engine</label>
                        <input 
                          type="text" 
                          value={llmModel} 
                          onChange={e => setLlmModel(e.target.value)} 
                          className="control-input" 
                          placeholder={llmProvider === 'Gemini' ? "gemini-1.5-pro" : "gpt-oss"} 
                        />
                      </div>
                      
                      <button onClick={() => setShowAgentSettings(false)} className="primary-btn" style={{ padding: '8px 16px', width: 'auto' }}>
                        Save & Close
                      </button>
                    </div>
                  )}

                  <div className="chat-messages">
                    {messages.map((msg) => (
                      <div key={msg.id} className={`message ${msg.sender}`}>
                        {renderMessage(msg.text)}
                      </div>
                    ))}
                    {isLoading && (
                      <div className="message agent" style={{ color: '#747775', fontStyle: 'italic' }}>
                        Agent is thinking and querying via {llmProvider}...
                      </div>
                    )}
                  </div>
                  <div className="chat-input-area">
                    <textarea
                      className="chat-input"
                      rows={2}
                      placeholder='Ask me a question...'
                      value={inputValue}
                      onChange={(e) => setInputValue(e.target.value)}
                      onKeyDown={(e) => {
                        if(e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSendMessage(); }
                      }}
                    />
                  </div>
                </div>
              </>
            )}

            {activeTab === 'db_explorer' && (
              <div style={{ padding: '32px', width: '100%', display: 'flex', flexDirection: 'column', gap: '20px' }}>
                <div style={{ backgroundColor: '#e8f0fe', padding: '20px', borderRadius: '12px', color: '#041e49', border: '1px solid #d3e3fd' }}>
                  <strong>LLM Context Fetcher:</strong> Use this tool to query the live SDSC Postgres database and copy the JSON schema structures back to me.
                </div>

                <div>
                  <div style={{ fontSize: '14px', fontWeight: 500, marginBottom: '8px', color: '#444746' }}>Quick Explore Tables:</div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                    {quickTables.map((t) => (
                      <button 
                        key={t} 
                        onClick={() => { setExploreTable(t); }} 
                        style={{ 
                          padding: '6px 14px', 
                          borderRadius: '16px', 
                          border: '1px solid #0b57d0', 
                          background: exploreTable === t ? '#0b57d0' : '#ffffff', 
                          color: exploreTable === t ? '#ffffff' : '#0b57d0',
                          cursor: 'pointer', 
                          fontSize: '13px',
                          transition: 'all 0.2s'
                        }}
                      >
                        {t}
                      </button>
                    ))}
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-end', marginTop: '8px' }}>
                  <div style={{ flex: 1, maxWidth: '400px' }}>
                    <label style={{ display: 'block', fontSize: '14px', fontWeight: 500, marginBottom: '8px' }}>Target Table Name</label>
                    <input 
                      type="text" 
                      className="control-input" 
                      value={exploreTable} 
                      onChange={(e) => setExploreTable(e.target.value)} 
                      placeholder="e.g. nourish_cbg_food_environment"
                    />
                  </div>
                  <button className="primary-btn" style={{ width: 'auto' }} onClick={handleExploreDB}>
                    Fetch Schema
                  </button>
                </div>
                <textarea 
                  readOnly 
                  style={{ flex: 1, width: '100%', fontFamily: 'monospace', padding: '20px', border: '1px solid #e0e0e0', borderRadius: '12px', resize: 'none', backgroundColor: '#f8f9fa' }}
                  value={exploreResult}
                />
              </div>
            )}
          </div>
        </main>
      </div>
    </>
  );
}
