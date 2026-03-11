import { useState, useEffect, useRef } from 'react';
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

type DetailedCosts = {
  estimatedRent: number | null;
  estimatedUtilities: number | null;
  laborCostPct: number | null;
  source: string;
};

type Demographics = {
  incomeLevel: number | null;
  gentrificationIndicator: number | null;
  targetPopulationGrowth: number | null;
  foodDesertStatus: boolean;
  lowIncomeLowAccess: boolean;
  foodInsecurityRate: number | null;
  source: string;
};

type LocationEvalResponse = {
  lat: number;
  lng: number;
  opportunityScore: number;
  footTraffic: number | null;
  footTrafficSource: string;
  isApproximated: boolean;
  nearbyCompetitors: number;
  supportiveBusinesses: number;
  demographics: Demographics;
  operatingCosts: DetailedCosts;
  demographicProfile: string;
  reviewCount: number;
  statsExtra: string;
  calcLog: string;
  citywideActiveTaxCompetitor: number;
};

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
  const [activeTab, setActiveTab] = useState<'home' | 'map' | 'recommend' | 'db_explorer'>('map');
  const [activeWorkspace, setActiveWorkspace] = useState('General Food Business');
  const [messages, setMessages] = useState<Message[]>([
    { id: 1, sender: 'agent', text: 'Hello. I am the Nourish PT Data Agent backed by the AI Gateway. How can I help you find gaps in the market today?' }
  ]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const[activeLayers, setActiveLayers] = useState<string[]>(['base_map']);
  
  const[naicsFilter, setNaicsFilter] = useState('445');
  const [mapPoints, setMapPoints] = useState<MapData[]>([]);
  const [debugInfo, setDebugInfo] = useState<any>(null);
  const [businessProfiles, setBusinessProfiles] = useState<any[]>([]);

  // General Config
  const [allowApproximations, setAllowApproximations] = useState(true);
  const[computationMethod, setComputationMethod] = useState('standard');
  const [liveCalculation, setLiveCalculation] = useState(true);
  const [showLiveWarning, setShowLiveWarning] = useState(false);
  const searchHistoryRef = useRef<number[]>([]);
  
  // Custom Scoring Profiles
  const[scoringProfile, setScoringProfile] = useState('standard');
  const [customWeights, setCustomWeights] = useState({
    traffic: 1.0,
    compPenalty: 8.0,
    suppBonus: 1.5,
    costPenalty: 5.0,
    ratingBonus: 15.0,
    foodDesertBonus: 0.0,
    gentrificationWeight: 0.0
  });

  useEffect(() => {
    fetch('http://localhost:8081/api/business-profiles')
      .then(res => res.json())
      .then(data => setBusinessProfiles(data))
      .catch(err => console.error(err));
  },[]);

  useEffect(() => {
     setSelectedLocation(null);
  }, [activeTab]);

  const handleProfileChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    setScoringProfile(val);
    if (val === 'standard') {
      setCustomWeights({ traffic: 1.0, compPenalty: 8.0, suppBonus: 1.5, costPenalty: 5.0, ratingBonus: 15.0, foodDesertBonus: 0.0, gentrificationWeight: 0.0 });
    } else if (val === 'traffic_heavy') {
      setCustomWeights({ traffic: 2.5, compPenalty: 5.0, suppBonus: 2.0, costPenalty: 3.0, ratingBonus: 10.0, foodDesertBonus: 0.0, gentrificationWeight: 0.0 });
    } else if (val === 'cost_averse') {
      setCustomWeights({ traffic: 1.0, compPenalty: 8.0, suppBonus: 1.5, costPenalty: 12.0, ratingBonus: 8.0, foodDesertBonus: 0.0, gentrificationWeight: 0.0 });
    } else if (val === 'offset_food_deserts') {
      setCustomWeights({ traffic: 1.5, compPenalty: 12.0, suppBonus: 2.5, costPenalty: 7.0, ratingBonus: 5.0, foodDesertBonus: 30.0, gentrificationWeight: -5.0 });
    }
  };

  const handleWeightChange = (field: keyof typeof customWeights, value: string) => {
    setCustomWeights(prev => ({...prev,[field]: parseFloat(value) || 0}));
  };

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

  const [heatmapMode, setHeatmapMode] = useState(true);

  const[mapBounds, setMapBounds] = useState<{n: number, s: number, e: number, w: number} | null>({
    n: 32.95, s: 32.65, e: -116.95, w: -117.30
  });
  
  const [selectedLocation, setSelectedLocation] = useState<{lat: number, lng: number} | null>(null);
  const [locationEval, setLocationEval] = useState<LocationEvalResponse | null>(null);
  const [isEvaluating, setIsEvaluating] = useState(false);

  const[recommendations, setRecommendations] = useState<any[]>([]);
  const [isRecommending, setIsRecommending] = useState(false);

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

    if (activeTab === 'recommend') {
      setIsRecommending(true);
      setRecommendations([]);
      try {
        const response = await fetch(`http://localhost:8081/api/recommend-business?lat=${lat}&lng=${lng}`);
        const data = await response.json();
        setRecommendations(data ||[]);
      } catch (error) {
        console.error("Recommendation failed", error);
      } finally {
        setIsRecommending(false);
      }
    } else {
      setIsEvaluating(true);
      setLocationEval(null);

      try {
        const response = await fetch(`http://localhost:8081/api/evaluate-location?lat=${lat}&lng=${lng}&naics=${naicsFilter}&allowApproximations=${allowApproximations}&computationMethod=${computationMethod}&trafficW=${customWeights.traffic}&compW=${customWeights.compPenalty}&suppW=${customWeights.suppBonus}&costW=${customWeights.costPenalty}&ratingW=${customWeights.ratingBonus}&foodDesertW=${customWeights.foodDesertBonus}&gentrificationW=${customWeights.gentrificationWeight}`);
        const data = await response.json();
        setLocationEval(data);
      } catch (error) {
        console.error("Evaluation failed", error);
      } finally {
        setIsEvaluating(false);
      }
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
    const now = Date.now();
    searchHistoryRef.current = searchHistoryRef.current.filter(t => now - t < 10000); 
    
    if (searchHistoryRef.current.length >= 4 && liveCalculation) {
      setShowLiveWarning(true);
    }
    searchHistoryRef.current.push(now);

    setIsLoading(true);
    try {
      let url = `http://localhost:8081/api/opportunity-map?naics=${naicsFilter}&allowApproximations=${allowApproximations}&computationMethod=${computationMethod}&trafficW=${customWeights.traffic}&compW=${customWeights.compPenalty}&suppW=${customWeights.suppBonus}&costW=${customWeights.costPenalty}&ratingW=${customWeights.ratingBonus}&foodDesertW=${customWeights.foodDesertBonus}&gentrificationW=${customWeights.gentrificationWeight}`;
      if (mapBounds) {
        url += `&n=${mapBounds.n}&s=${mapBounds.s}&e=${mapBounds.e}&w=${mapBounds.w}`;
      }
      
      const response = await fetch(url);
      const payload = await response.json();
      setMapPoints(payload.data?.points ||[]);
      setDebugInfo(payload.data?.debug || null);
      setActiveLayers([
        `NAICS Prefix: ${naicsFilter}`
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
      if (!liveCalculation) return;

      const delayDebounceFn = setTimeout(() => {
        handleManualSearch();
      }, 1000);

      return () => clearTimeout(delayDebounceFn);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[naicsFilter, allowApproximations, computationMethod, customWeights, mapBounds, activeTab, liveCalculation]);

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
          <div className={`sidebar-item ${activeTab === 'map' ? 'active' : ''}`} onClick={() => setActiveTab('map')}>Opportunity Map (By Business)</div>
          <div className={`sidebar-item ${activeTab === 'recommend' ? 'active' : ''}`} onClick={() => setActiveTab('recommend')}>Location Recommender</div>
          <div className={`sidebar-item ${activeTab === 'db_explorer' ? 'active' : ''}`} onClick={() => setActiveTab('db_explorer')}>Database Explorer</div>
          
          <div className="sidebar-header" style={{marginTop: '16px'}}>Integration API Docs</div>
          <div className="sidebar-item" onClick={() => window.open('http://localhost:8081/swagger', '_blank')} style={{color: '#0b57d0'}}>
            Swagger / OpenAPI ↗
          </div>

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
              {activeTab === 'recommend' && 'Location Recommender System'}
              {activeTab === 'db_explorer' && 'Database Schema Explorer'}
            </h1>
          </div>
          
          <div className="workspace-body">
            {activeTab === 'home' && (
              <div className="home-container">
                <div className="home-card">
                  <h2>Welcome to the Nourish PT Platform</h2>
                  <p>This application helps identify the optimal streets and block groups in San Diego County to establish new food businesses. It queries live data directly from the PostgreSQL data warehouse to find market gaps, using a multi-dimensional computation methodology that accommodates variables such as food deserts, rental costs, and competitive penalty bounds.</p>
                  
                  <h3>Two Core Functions</h3>
                  <ul>
                    <li><strong>Start from Business Type:</strong> Select a business structure (e.g. Healthy Grocery) and our engine highlights the optimal parcels.</li>
                    <li><strong>Start from Location:</strong> Click anywhere on our map and the system recommends the most mathematically viable NAICS entity to open.</li>
                  </ul>

                  <h3>The Opportunity Scoring Methodology</h3>
                  <div className="equation-box">
                    Opportunity Score = Base (45) + Foot Traffic Impact + Supportive Biz Bonus - Cost Penalties - Competition Penalties + Market Gap Bonus + Food Desert Offset
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
                  <h2 className="panel-title">Scoring Function Selection</h2>

                  <div className="control-group">
                    <label>Business Vertical / Goal Profile</label>
                    <select className="control-select" value={naicsFilter} onChange={e => {
                        const profile = businessProfiles.find(p => p.naics === e.target.value);
                        setNaicsFilter(e.target.value);
                        if(profile) {
                            setCustomWeights({
                                traffic: profile.trafficWeight,
                                compPenalty: profile.compPenaltyWeight,
                                suppBonus: profile.suppBonusWeight,
                                costPenalty: profile.costPenaltyWeight,
                                ratingBonus: profile.ratingBonusWeight,
                                foodDesertBonus: profile.foodDesertBonus,
                                gentrificationWeight: profile.gentrificationWeight
                            });
                            setScoringProfile('custom');
                        }
                    }}>
                      {businessProfiles.map(p => (
                          <option key={p.naics} value={p.naics}>{p.name} ({p.naics})</option>
                      ))}
                      {businessProfiles.length === 0 && (
                          <>
                            <option value="445">Food and Beverage Stores (445)</option>
                            <option value="722">Food Services and Drinking (722)</option>
                            <option value="454">Nonstore Retailers / Food Trucks (454)</option>
                          </>
                      )}
                    </select>
                  </div>

                  <div className="control-group">
                    <label>Active Scoring Filter Mode</label>
                    <select className="control-select" value={scoringProfile} onChange={handleProfileChange}>
                      <option value="standard">Standard Balanced Approach</option>
                      <option value="traffic_heavy">Prioritize Foot Traffic (Pedestrian Heavy)</option>
                      <option value="cost_averse">Cost Averse (Penalty for High Rent)</option>
                      <option value="offset_food_deserts">Community First (Offset Food Deserts)</option>
                      <option value="custom">⚙️ Custom Math Profile</option>
                    </select>
                  </div>

                  {scoringProfile === 'custom' && (
                    <div style={{ backgroundColor: '#f0f4f9', padding: '12px', borderRadius: '8px', marginBottom: '16px', border: '1px solid #d3e3fd' }}>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Traffic Positivity Weight</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.traffic} onChange={e => handleWeightChange('traffic', e.target.value)} />
                      </div>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Competitor Penalty Multiplier</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.compPenalty} onChange={e => handleWeightChange('compPenalty', e.target.value)} />
                      </div>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Supportive Biz Bonus Mutliplier</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.suppBonus} onChange={e => handleWeightChange('suppBonus', e.target.value)} />
                      </div>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Cost/Rent Penalty Multiplier</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.costPenalty} onChange={e => handleWeightChange('costPenalty', e.target.value)} />
                      </div>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Food Desert Offset Bonus</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.foodDesertBonus} onChange={e => handleWeightChange('foodDesertBonus', e.target.value)} />
                      </div>
                      <div className="control-group" style={{ marginBottom: '8px' }}>
                        <label style={{ fontSize: '12px' }}>Gentrification Weight (Income Proxy)</label>
                        <input type="number" step="0.5" className="control-input" value={customWeights.gentrificationWeight} onChange={e => handleWeightChange('gentrificationWeight', e.target.value)} />
                      </div>
                    </div>
                  )}

                  <hr style={{margin: '16px 0', borderColor: '#e0e0e0'}} />

                  <h2 className="panel-title">Data Filter Configuration</h2>

                  <div className="control-group">
                    <label>Computation Method / Search Area Strategy</label>
                    <select className="control-select" value={computationMethod} onChange={e => setComputationMethod(e.target.value)}>
                      <option value="standard">Standard Local Allocation (Dense Focus)</option>
                      <option value="boutique">Boutique & Additive (Larger Trade Area)</option>
                    </select>
                  </div>
                  
                  <div className="control-group" style={{ backgroundColor: showLiveWarning ? '#fce8e6' : 'transparent', padding: showLiveWarning ? '8px' : '0', borderRadius: '8px', transition: 'all 0.3s' }}>
                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', color: showLiveWarning ? '#b2182b' : 'inherit' }}>
                      <input 
                        type="checkbox" 
                        checked={liveCalculation} 
                        onChange={e => {
                          setLiveCalculation(e.target.checked);
                          if(e.target.checked === false) setShowLiveWarning(false);
                        }} 
                      />
                      Enable Live Dynamic Calculations
                    </label>
                    {showLiveWarning && (
                      <div style={{ fontSize: '12px', color: '#b2182b', marginTop: '6px' }}>
                        ⚠️ Rapid calculations detected. Consider turning this off to limit heavy data processing while adjusting map bounds.
                      </div>
                    )}
                  </div>

                  <div className="control-group">
                    <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                      <input type="checkbox" checked={allowApproximations} onChange={e => setAllowApproximations(e.target.checked)} />
                      Allow Proxy Estimation / Fallback Math (Enable for Counterfactuals)
                    </label>
                  </div>

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

                <div className="map-container" style={{ position: 'relative' }}>
                  {!liveCalculation && (
                    <button 
                      onClick={handleManualSearch}
                      style={{
                        position: 'absolute', top: 16, left: '50%', transform: 'translateX(-50%)',
                        zIndex: 2000, padding: '10px 24px', backgroundColor: '#0b57d0', color: 'white',
                        borderRadius: '24px', border: 'none', fontWeight: 500, cursor: 'pointer',
                        boxShadow: '0 2px 6px rgba(0,0,0,0.3)'
                      }}
                    >
                      🔄 Run Calculation For Current Area
                    </button>
                  )}

                  <div className="map-overlay" style={{ top: !liveCalculation ? '64px' : '16px' }}>
                    <strong>Highest Score Highlighted: {maxScore > 0 ? maxScore : '...'}</strong>
                    {activeLayers.map((layer, i) => <div key={i} style={{color: '#0b57d0', fontSize: '13px', marginTop: '4px'}}>{layer}</div>)}
                    <div style={{marginTop: '8px', color: '#444746'}}>
                      {heatmapMode ? 'Showing Canvas-Rendered Gradient Heatmap.' : 'Showing Precise Plot Marker Points.'}<br/>
                      <em>Tip: Click any marker to view real data & economics.</em>
                    </div>
                  </div>

                  {selectedLocation && (
                    <div className="evaluation-panel" style={{ width: '380px', maxHeight: '90%', overflowY: 'auto' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                        <h3 style={{ margin: 0 }}>📍 Enterprise Location Eval</h3>
                        <button onClick={() => setSelectedLocation(null)} style={{ border: 'none', background: 'none', cursor: 'pointer', fontSize: '18px' }}>✖</button>
                      </div>
                      
                      {isEvaluating ? (
                        <p style={{fontSize: '14px', color: '#444746'}}>Running database queries on coordinates...</p>
                      ) : locationEval ? (
                        <>
                          <div className="eval-metric">
                            <span>Area Foot Traffic:</span>
                            <span>{locationEval.footTraffic ?? 'Strict NULL'} {locationEval.isApproximated ? '(Proxy)' : ''}</span>
                          </div>
                          <div className="eval-metric">
                            <span>Direct Competitors:</span>
                            <span style={{ color: '#b2182b', fontWeight: 'bold' }}>{locationEval.nearbyCompetitors}</span>
                          </div>
                          <div className="eval-metric">
                            <span>Supportive / Related Biz:</span>
                            <span style={{ color: '#0f9d58', fontWeight: 'bold' }}>{locationEval.supportiveBusinesses}</span>
                          </div>
                          
                          <h4 style={{ margin: '12px 0 8px 0', fontSize: '13px', color: '#444746' }}>Demographics & Indicators</h4>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Income Level (Est):</span>
                            <span>{locationEval.demographics.incomeLevel ? `$${locationEval.demographics.incomeLevel.toLocaleString()}` : 'N/A'}</span>
                          </div>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Gentrification Index:</span>
                            <span style={{ color: locationEval.demographics.gentrificationIndicator && locationEval.demographics.gentrificationIndicator > 0 ? '#0f9d58' : 'inherit' }}>
                              {locationEval.demographics.gentrificationIndicator ? `+${locationEval.demographics.gentrificationIndicator.toFixed(1)}%` : 'N/A'}
                            </span>
                          </div>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Population Growth:</span>
                            <span>{locationEval.demographics.targetPopulationGrowth ? `+${locationEval.demographics.targetPopulationGrowth.toFixed(1)}%` : 'N/A'}</span>
                          </div>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>USDA Food Desert:</span>
                            <span>{locationEval.demographics.foodDesertStatus ? 'Yes (System Aware)' : 'No'}</span>
                          </div>

                          <h4 style={{ margin: '12px 0 8px 0', fontSize: '13px', color: '#444746' }}>Operating Cost Estimates (SBA Guidelines)</h4>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Rent Baseline (~sqft/yr):</span>
                            <span>{locationEval.operatingCosts.estimatedRent ? `$${locationEval.operatingCosts.estimatedRent}` : 'Unknown'}</span>
                          </div>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Est. Utilities (/mo):</span>
                            <span>{locationEval.operatingCosts.estimatedUtilities ? `$${locationEval.operatingCosts.estimatedUtilities.toFixed(0)}` : 'Unknown'}</span>
                          </div>
                          <div className="eval-metric" style={{ fontSize: '13px' }}>
                            <span>Est. Labor Load (% Rev):</span>
                            <span>{locationEval.operatingCosts.laborCostPct ? `${locationEval.operatingCosts.laborCostPct}%` : 'Unknown'}</span>
                          </div>
                          
                          <div style={{ marginTop: '16px', padding: '8px', backgroundColor: '#f0f4f9', borderRadius: '8px', fontSize: '11px', fontFamily: 'monospace', color: '#041e49', wordBreak: 'break-all' }}>
                            <strong>Calc Trace:</strong><br/>
                            {locationEval.calcLog}
                          </div>

                          <div className="eval-metric" style={{marginTop: '16px', border: 'none', fontSize: '18px'}}>
                            <span><strong>Opp. Score (0-100):</strong></span>
                            <span style={{color: locationEval.opportunityScore > 70 ? '#0f9d58' : locationEval.opportunityScore < 30 ? '#444746' : '#db4437'}}>
                              {locationEval.opportunityScore.toFixed(1)}
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

            {activeTab === 'recommend' && (
              <>
                <div className="manual-panel">
                  <h2 className="panel-title">Location Recommender</h2>
                  <p style={{ fontSize: '14px', color: '#444746', marginBottom: '16px', lineHeight: '1.6' }}>
                    Click anywhere on the map to evaluate a specific point or neighborhood against our computational framework.
                    It will automatically process all available business configurations (NAICS structures) and recommend the best fit based on market gaps, local competition, demographic bonuses, and land costs.
                  </p>
                  
                  {selectedLocation && (
                    <div style={{ backgroundColor: '#f0f4f9', padding: '16px', borderRadius: '8px', border: '1px solid #d3e3fd' }}>
                      <h3 style={{ fontSize: '14px', marginBottom: '12px', color: '#041e49' }}>📍 Selected Coordinates</h3>
                      <div style={{ fontSize: '13px', fontFamily: 'monospace', marginBottom: '16px' }}>
                        Lat: {selectedLocation.lat.toFixed(5)}<br/>
                        Lng: {selectedLocation.lng.toFixed(5)}
                      </div>

                      {isRecommending ? (
                        <div style={{ fontSize: '13px', color: '#747775' }}>Computing cross-profile evaluations...</div>
                      ) : recommendations.length > 0 ? (
                        <div>
                          <h4 style={{ fontSize: '13px', marginBottom: '8px', color: '#1f1f1f' }}>Top Recommended Models:</h4>
                          {recommendations.map((rec, i) => (
                            <div key={i} style={{ backgroundColor: 'white', padding: '12px', borderRadius: '6px', marginBottom: '8px', borderLeft: `4px solid ${i === 0 ? '#0f9d58' : '#0b57d0'}`, boxShadow: '0 1px 2px rgba(0,0,0,0.05)' }}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                                <strong style={{ fontSize: '13px' }}>{rec.profile.name}</strong>
                                <span style={{ fontWeight: 'bold', color: rec.score > 70 ? '#0f9d58' : '#1f1f1f' }}>{rec.score.toFixed(1)}</span>
                              </div>
                              <div style={{ fontSize: '11px', color: '#747775' }}>NAICS Framework: {rec.profile.naics}</div>
                              <div style={{ fontSize: '11px', color: '#444746', marginTop: '6px', lineHeight: '1.4' }}>{rec.details}</div>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div style={{ fontSize: '13px', color: '#747775' }}>No recommendations generated for this point.</div>
                      )}
                    </div>
                  )}
                </div>
                <div className="map-container" style={{ position: 'relative' }}>
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
                    {selectedLocation && (
                      <CircleMarker 
                        center={[selectedLocation.lat, selectedLocation.lng]} 
                        radius={8} 
                        pathOptions={{ color: '#000000', weight: 2, fillColor: '#ffffff', fillOpacity: 1 }}
                      />
                    )}
                  </MapContainer>
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

