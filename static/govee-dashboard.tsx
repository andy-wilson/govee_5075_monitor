import React, { useState, useEffect } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, BarChart, Bar } from 'recharts';

const Dashboard = () => {
  const [dashboardData, setDashboardData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedDevice, setSelectedDevice] = useState(null);
  const [refreshInterval, setRefreshInterval] = useState(30);
  const [lastUpdated, setLastUpdated] = useState(null);

  // Fetch dashboard data
  const fetchDashboardData = async () => {
    try {
      const response = await fetch('/dashboard/data');
      if (!response.ok) {
        throw new Error(`Server responded with ${response.status}`);
      }
      const data = await response.json();
      setDashboardData(data);
      
      // Select first device as default if none selected
      if (!selectedDevice && data.devices.length > 0) {
        setSelectedDevice(data.devices[0].DeviceAddr);
      }
      
      setLastUpdated(new Date());
      setLoading(false);
    } catch (err) {
      setError(`Failed to fetch dashboard data: ${err.message}`);
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    fetchDashboardData();
  }, []);

  // Set up periodic refresh
  useEffect(() => {
    const interval = setInterval(() => {
      fetchDashboardData();
    }, refreshInterval * 1000);
    
    return () => clearInterval(interval);
  }, [refreshInterval]);

  // Format time for display
  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
  };

  // Format date for display
  const formatDate = (timestamp) => {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleDateString();
  };

  // Get readings data for selected device
  const getDeviceReadings = () => {
    if (!dashboardData || !selectedDevice || !dashboardData.recent_readings[selectedDevice]) {
      return [];
    }
    
    return dashboardData.recent_readings[selectedDevice].map(reading => ({
      time: formatTime(reading.timestamp),
      temperature: reading.temp_c,
      humidity: reading.humidity,
      timestamp: reading.timestamp
    }));
  };

  // Get the selected device object
  const getSelectedDeviceObject = () => {
    if (!dashboardData || !selectedDevice) return null;
    return dashboardData.devices.find(d => d.DeviceAddr === selectedDevice);
  };

  // Check if a client is active (seen in the last 5 minutes)
  const isClientActive = (client) => {
    return client.is_active;
  };

  // Display system status
  const StatusSection = () => (
    <div className="bg-white p-6 rounded-lg shadow-md">
      <h2 className="text-xl font-semibold mb-4">System Status</h2>
      
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-blue-50 p-4 rounded">
          <div className="text-sm text-gray-500">Total Devices</div>
          <div className="text-2xl font-bold">{dashboardData?.devices.length || 0}</div>
        </div>
        
        <div className="bg-green-50 p-4 rounded">
          <div className="text-sm text-gray-500">Active Clients</div>
          <div className="text-2xl font-bold">{dashboardData?.active_clients || 0}</div>
        </div>
        
        <div className="bg-purple-50 p-4 rounded">
          <div className="text-sm text-gray-500">Total Readings</div>
          <div className="text-2xl font-bold">{dashboardData?.total_readings || 0}</div>
        </div>
        
        <div className="bg-yellow-50 p-4 rounded">
          <div className="text-sm text-gray-500">Last Updated</div>
          <div className="text-xl font-bold">{lastUpdated ? formatTime(lastUpdated) : '-'}</div>
        </div>
      </div>
    </div>
  );

  // Device selection and display
  const DeviceSection = () => {
    const selectedDeviceObj = getSelectedDeviceObject();
    
    return (
      <div className="bg-white p-6 rounded-lg shadow-md mt-6">
        <h2 className="text-xl font-semibold mb-4">Device Information</h2>
        
        <div className="mb-4">
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Select Device
          </label>
          <select 
            className="block w-full rounded-md border-gray-300 shadow-sm p-2 border"
            value={selectedDevice || ''}
            onChange={(e) => setSelectedDevice(e.target.value)}
          >
            <option value="">Select a device</option>
            {dashboardData?.devices.map((device) => (
              <option key={device.DeviceAddr} value={device.DeviceAddr}>
                {device.DeviceName} ({device.DeviceAddr})
              </option>
            ))}
          </select>
        </div>
        
        {selectedDeviceObj && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-gray-50 p-4 rounded">
              <div className="text-sm text-gray-500">Temperature</div>
              <div className="text-2xl font-bold">{selectedDeviceObj.TempC.toFixed(1)}°C / {selectedDeviceObj.TempF.toFixed(1)}°F</div>
            </div>
            
            <div className="bg-gray-50 p-4 rounded">
              <div className="text-sm text-gray-500">Humidity</div>
              <div className="text-2xl font-bold">{selectedDeviceObj.Humidity.toFixed(1)}%</div>
            </div>
            
            <div className="bg-gray-50 p-4 rounded">
              <div className="text-sm text-gray-500">Battery</div>
              <div className="text-2xl font-bold">{selectedDeviceObj.Battery}%</div>
            </div>
            
            <div className="bg-gray-50 p-4 rounded">
              <div className="text-sm text-gray-500">Signal Strength</div>
              <div className="text-2xl font-bold">{selectedDeviceObj.RSSI} dBm</div>
            </div>
            
            <div className="bg-gray-50 p-4 rounded col-span-2">
              <div className="text-sm text-gray-500">Last Update</div>
              <div className="text-xl font-bold">
                {formatDate(selectedDeviceObj.LastUpdate)} {formatTime(selectedDeviceObj.LastUpdate)}
              </div>
            </div>
            
            <div className="bg-gray-50 p-4 rounded col-span-2">
              <div className="text-sm text-gray-500">Client</div>
              <div className="text-xl font-bold">{selectedDeviceObj.ClientID}</div>
            </div>
          </div>
        )}
      </div>
    );
  };

  // Chart section
  const ChartSection = () => {
    const readings = getDeviceReadings();
    const selectedDeviceObj = getSelectedDeviceObject();
    
    if (!selectedDeviceObj || readings.length === 0) {
      return (
        <div className="bg-white p-6 rounded-lg shadow-md mt-6">
          <h2 className="text-xl font-semibold mb-4">Temperature & Humidity Charts</h2>
          <p className="text-gray-500">Select a device to view charts</p>
        </div>
      );
    }
    
    return (
      <div className="bg-white p-6 rounded-lg shadow-md mt-6">
        <h2 className="text-xl font-semibold mb-4">Temperature & Humidity Charts</h2>
        
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="h-64">
            <h3 className="text-lg font-medium mb-2">Temperature (°C)</h3>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={readings}
                margin={{ top: 5, right: 30, left: 0, bottom: 5 }}
              >
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis domain={['auto', 'auto']} />
                <Tooltip />
                <Legend />
                <Line 
                  type="monotone" 
                  dataKey="temperature" 
                  stroke="#8884d8" 
                  activeDot={{ r: 8 }} 
                  name="Temperature (°C)"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
          
          <div className="h-64">
            <h3 className="text-lg font-medium mb-2">Humidity (%)</h3>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={readings}
                margin={{ top: 5, right: 30, left: 0, bottom: 5 }}
              >
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis domain={[0, 100]} />
                <Tooltip />
                <Legend />
                <Line 
                  type="monotone" 
                  dataKey="humidity" 
                  stroke="#82ca9d" 
                  activeDot={{ r: 8 }} 
                  name="Humidity (%)"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    );
  };

  // Clients section
  const ClientsSection = () => {
    if (!dashboardData || !dashboardData.clients) {
      return null;
    }
    
    return (
      <div className="bg-white p-6 rounded-lg shadow-md mt-6">
        <h2 className="text-xl font-semibold mb-4">Connected Clients</h2>
        
        <div className="overflow-x-auto">
          <table className="min-w-full bg-white">
            <thead className="bg-gray-100">
              <tr>
                <th className="py-2 px-4 text-left">Client ID</th>
                <th className="py-2 px-4 text-left">Status</th>
                <th className="py-2 px-4 text-left">Devices</th>
                <th className="py-2 px-4 text-left">Readings</th>
                <th className="py-2 px-4 text-left">Last Seen</th>
                <th className="py-2 px-4 text-left">Connected Since</th>
              </tr>
            </thead>
            <tbody>
              {dashboardData.clients.map((client) => (
                <tr key={client.ClientID} className="border-t">
                  <td className="py-2 px-4">{client.ClientID}</td>
                  <td className="py-2 px-4">
                    <span className={`px-2 py-1 rounded text-xs font-semibold ${client.IsActive ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
                      {client.IsActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="py-2 px-4">{client.DeviceCount}</td>
                  <td className="py-2 px-4">{client.ReadingCount}</td>
                  <td className="py-2 px-4">{formatTime(client.LastSeen)}</td>
                  <td className="py-2 px-4">{formatDate(client.ConnectedSince)} {formatTime(client.ConnectedSince)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  // Refresh control
  const RefreshControl = () => (
    <div className="flex items-center space-x-4 mt-6">
      <span className="text-sm text-gray-500">Auto-refresh every:</span>
      <select 
        className="rounded border-gray-300 p-1 text-sm"
        value={refreshInterval}
        onChange={(e) => setRefreshInterval(Number(e.target.value))}
      >
        <option value="10">10 seconds</option>
        <option value="30">30 seconds</option>
        <option value="60">1 minute</option>
        <option value="300">5 minutes</option>
      </select>
      <button 
        className="bg-blue-500 text-white px-4 py-1 rounded text-sm"
        onClick={fetchDashboardData}
      >
        Refresh Now
      </button>
    </div>
  );

  if (loading) {
    return <div className="p-6 text-center">Loading dashboard data...</div>;
  }

  if (error) {
    return <div className="p-6 text-center text-red-500">{error}</div>;
  }

  return (
    <div className="container mx-auto p-4">
      <div className="bg-blue-700 text-white p-4 rounded-lg shadow-md mb-6">
        <h1 className="text-2xl font-bold">Govee Temperature & Humidity Dashboard</h1>
        <p className="opacity-80">Real-time monitoring of Govee H5075 sensors</p>
      </div>
      
      <StatusSection />
      <DeviceSection />
      <ChartSection />
      <ClientsSection />
      <RefreshControl />
    </div>
  );
};

export default Dashboard;