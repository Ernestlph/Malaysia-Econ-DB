// --- DOM Elements ---
const priceChartContainer = document.getElementById('price-chart-container');
const returnChartContainer = document.getElementById('return-chart-container');
const loadingIndicators = document.querySelectorAll('.loading-indicator');
const dataTypeSelect = document.getElementById('dataType');
const dataCodeInput = document.getElementById('dataCode');
const addButton = document.getElementById('addButton');
const startDateInput = document.getElementById('startDate');
const endDateInput = document.getElementById('endDate');
const reloadButton = document.getElementById('reloadButton');
const activeSeriesListDiv = document.getElementById('activeSeriesList');
const statsDisplayDiv = document.getElementById('statsDisplay'); // For correlation

// --- Chart Instances ---
let priceChart = null;
let returnChart = null;

// --- State Management ---
let activeSeries = {};
let nextPriceScaleId = 'right';

// --- Default Dates ---
const today = new Date();
const oneYearAgo = new Date();
oneYearAgo.setFullYear(today.getFullYear() - 1);
endDateInput.value = today.toISOString().split('T')[0];
startDateInput.value = oneYearAgo.toISOString().split('T')[0];

// --- Series Type Mapping for Lightweight Charts v5 addSeries method ---
const seriesDefinitionMap = {
    'stock': () => LightweightCharts.LineSeries,
    'fx': () => LightweightCharts.LineSeries,
};

// --- Calculation Functions ---
function calculateSMA(priceData, period) {
    if (!priceData || priceData.length < period) return [];
    const smaValues = [];
    let currentSum = 0;
    for (let i = 0; i < period; i++) currentSum += priceData[i].value;
    smaValues.push(currentSum / period);
    for (let i = period; i < priceData.length; i++) {
        currentSum -= priceData[i - period].value;
        currentSum += priceData[i].value;
        smaValues.push(currentSum / period);
    }
    const result = [];
    for (let i = 0; i < priceData.length; i++) {
        result.push({ time: priceData[i].time, value: i < period - 1 ? undefined : smaValues[i - (period - 1)] });
    }
    return result;
}

function calculateTotalReturn(priceData) {
    if (!priceData || priceData.length < 1) return [];
    const startPrice = priceData[0].value;
    if (startPrice <= 0) {
        console.warn("Cannot calculate total return with non-positive starting price.");
        return priceData.map(point => ({ time: point.time, value: undefined }));
    }
    return priceData.map(point => ({
        time: point.time,
        value: ((point.value - startPrice) / startPrice) * 100,
    }));
}

function calculateCorrelation(arr1, arr2) {
    if (arr1.length !== arr2.length || arr1.length < 2) { // Need at least 2 points
        return NaN;
    }
    const n = arr1.length;
    const sumX = arr1.reduce((a, b) => a + b, 0);
    const sumY = arr2.reduce((a, b) => a + b, 0);
    const meanX = sumX / n;
    const meanY = sumY / n;

    let sumXY = 0;
    let sumX2 = 0;
    let sumY2 = 0;

    for (let i = 0; i < n; i++) {
        sumXY += (arr1[i] - meanX) * (arr2[i] - meanY);
        sumX2 += Math.pow(arr1[i] - meanX, 2);
        sumY2 += Math.pow(arr2[i] - meanY, 2);
    }

    const denominator = Math.sqrt(sumX2 * sumY2);
    if (denominator === 0) {
        // This happens if one or both series have zero variance (e.g., all values are the same)
        // Could return 0 if you consider no variance as no linear relationship,
        // or NaN if you consider it undefined.
        return 0;
    }
    return sumXY / denominator;
}


// --- Helper Functions ---
function setLoading(isLoading) {
    loadingIndicators.forEach(el => el.style.display = isLoading ? 'block' : 'none');
}
function generateSeriesId(type, code) { return `${type}_${code}`; }
const colorPalette = ['#2962FF', '#E91E63', '#FF9800', '#4CAF50', '#9C27B0', '#00BCD4', '#F44336', '#795548', '#607D8B'];
let colorIndex = 0;
function getNextColor() {
    const color = colorPalette[colorIndex % colorPalette.length];
    colorIndex++;
    return color;
}
function getNextPriceScaleId() {
    const current = nextPriceScaleId;
    nextPriceScaleId = (current === 'right') ? 'left' : 'right';
    return current;
}
const apiEndpoints = {
    stock: (code) => `/api/stock/prices?code=${code}`,
    fx: (code) => `/api/fx/rates?code=${code}`,
};

// --- Data Fetching ---
async function fetchSeriesData(type, code, startDate, endDate) {
    const endpointBuilder = apiEndpoints[type];
    if (!endpointBuilder) throw new Error(`Unknown data type: ${type}`);
    const apiUrl = `${endpointBuilder(code)}&start_date=${startDate}&end_date=${endDate}`;
    console.log(`Fetching: ${apiUrl}`);
    const response = await fetch(apiUrl);

    if (!response.ok) {
        const errorText = await response.text();
        console.error(`API Error Response Body for ${type} ${code}:`, errorText);
        throw new Error(`HTTP error for ${type} ${code}! Status: ${response.status}.`);
    }
    const rawData = await response.json();

    if (!Array.isArray(rawData)) {
        console.error(`Invalid data format received for ${type} ${code}:`, rawData);
        throw new Error(`Data for ${type} ${code} is not an array.`);
    }
    console.log(`Received ${rawData.length} points for ${type} ${code}`);

    // --- Store additional info if available (like company_name) ---
    let fetchedCompanyName = code; // Default to code if no company_name
    if (rawData.length > 0 && rawData[0].company_name) { // Assuming company_name is consistent for all points in response
        fetchedCompanyName = rawData[0].company_name;
    }

    const processedData = rawData
        .map(item => ({
            time: item.date,
            value: typeof item.value === 'number' ? item.value : parseFloat(item.value)
        }))
        .filter(item => item.time && !isNaN(item.value))
        .sort((a, b) => new Date(a.time) - new Date(b.time));

    return { // Return an object containing processed data and any extra info
        dataPoints: processedData,
        companyName: fetchedCompanyName, // Pass along the company name
        // stockCode will be the 'code' parameter passed to this function
    };
}

// --- Chart Rendering ---
function renderCharts() {
    setLoading(true);
    if (priceChart) { priceChart.remove(); priceChart = null; }
    if (returnChart) { returnChart.remove(); returnChart = null; }
    priceChartContainer.innerHTML = '';
    returnChartContainer.innerHTML = '';
    Object.values(activeSeries).forEach(s => s.seriesRefs = {});

    if (Object.keys(activeSeries).length === 0) {
        priceChartContainer.innerHTML = 'Add a series to begin.';
        if (statsDisplayDiv) statsDisplayDiv.innerHTML = ''; // Clear stats
        setLoading(false);
        return;
    }

    try {
        priceChart = LightweightCharts.createChart(priceChartContainer, {
            width: priceChartContainer.clientWidth, height: priceChartContainer.clientHeight,
            layout: { background: { color: '#ffffff' }, textColor: '#333' },
            grid: { vertLines: { color: '#e1e1e1' }, horzLines: { color: '#e1e1e1' } },
            crosshair: { mode: LightweightCharts.CrosshairMode.Normal },
            timeScale: { timeVisible: true, secondsVisible: false, borderColor: '#e1e1e1' },
            leftPriceScale: { visible: true, borderColor: '#e1e1e1' },
            rightPriceScale: { visible: true, borderColor: '#e1e1e1' },
        });

        returnChart = LightweightCharts.createChart(returnChartContainer, {
            width: returnChartContainer.clientWidth, height: returnChartContainer.clientHeight,
            layout: { background: { color: '#ffffff' }, textColor: '#333' },
            grid: { vertLines: { color: '#e1e1e1' }, horzLines: { color: '#e1e1e1' } },
            crosshair: { mode: LightweightCharts.CrosshairMode.Normal },
            timeScale: { // MODIFIED: Make time scale visible and match priceChart
                timeVisible: true,
                secondsVisible: false,
                borderColor: '#e1e1e1',
            },
            rightPriceScale: { borderColor: '#e1e1e1', scaleMargins: { top: 0.2, bottom: 0.1 } },
        });

        if (!priceChart || typeof priceChart.addSeries !== 'function' || !returnChart || typeof returnChart.addSeries !== 'function') {
            throw new Error("Chart creation failed or addSeries method not found.");
        }
    } catch (chartError) {
        console.error("CRITICAL: Error creating chart instance(s):", chartError);
        priceChartContainer.innerHTML = `<div style="color: red; padding: 20px;">Error creating charts: ${chartError.message}.</div>`;
        if (statsDisplayDiv) statsDisplayDiv.innerHTML = ''; // Clear stats
        setLoading(false); priceChart = null; returnChart = null; return;
    }

    let firstSeriesId = null;
    for (const id in activeSeries) {
        const seriesInfo = activeSeries[id];
        if (!seriesInfo.data || seriesInfo.data.length === 0) continue;
        if (!firstSeriesId) firstSeriesId = id;

        const sma20Data = calculateSMA(seriesInfo.data, 20);
        const sma50Data = calculateSMA(seriesInfo.data, 50);
        const totalReturnData = calculateTotalReturn(seriesInfo.data); // Already calculated for return chart


        try {
            const getSeriesDef = seriesDefinitionMap[seriesInfo.type];
            if (!getSeriesDef) { console.error(`No series definition for type: ${seriesInfo.type}`); continue; }
            const seriesDefinition = getSeriesDef();
            const seriesTitle = seriesInfo.name || seriesInfo.code; // Fallback to code if name is empty

            seriesInfo.seriesRefs.price = priceChart.addSeries(seriesDefinition, { color: seriesInfo.color, lineWidth: 2, title: seriesTitle, priceScaleId: seriesInfo.priceScaleId });
            seriesInfo.seriesRefs.price.setData(seriesInfo.data);

            seriesInfo.seriesRefs.sma20 = priceChart.addSeries(LightweightCharts.LineSeries, { color: seriesInfo.color, lineStyle: LightweightCharts.LineStyle.Dashed, lineWidth: 1, title: `SMA20(${seriesTitle})`, priceScaleId: seriesInfo.priceScaleId, lastValueVisible: false, priceLineVisible: false });
            seriesInfo.seriesRefs.sma20.setData(sma20Data.filter(d => d.value !== undefined));

            seriesInfo.seriesRefs.sma50 = priceChart.addSeries(LightweightCharts.LineSeries, { color: seriesInfo.color, lineStyle: LightweightCharts.LineStyle.Dotted, lineWidth: 1, title: `SMA50(${seriesTitle})`, priceScaleId: seriesInfo.priceScaleId, lastValueVisible: false, priceLineVisible: false });
            seriesInfo.seriesRefs.sma50.setData(sma50Data.filter(d => d.value !== undefined));

            seriesInfo.seriesRefs.return = returnChart.addSeries(LightweightCharts.LineSeries, { color: seriesInfo.color, lineWidth: 1, title: `${seriesTitle} Ret%`, priceFormat: { type: 'percent', precision: 2, minMove: 0.01 }, lastValueVisible: true, priceLineVisible: true });
            seriesInfo.seriesRefs.return.setData(totalReturnData.filter(d => d.value !== undefined));
        } catch (seriesError) {
            console.error(`Error adding series data for ${id}:`, seriesError);
        }
    }

    // --- Correlation Calculation and Display ---
    if (statsDisplayDiv) statsDisplayDiv.innerHTML = ''; // Clear previous stats
    if (Object.keys(activeSeries).length === 2) {
        const seriesIds = Object.keys(activeSeries);
        const series1Info = activeSeries[seriesIds[0]];
        const series2Info = activeSeries[seriesIds[1]];

        // Use totalReturnData for correlation
        const returnData1 = calculateTotalReturn(series1Info.data);
        const returnData2 = calculateTotalReturn(series2Info.data);

        const pairedValues1 = [];
        const pairedValues2 = [];
        const returnData2Map = new Map();
        returnData2.forEach(p => { if (p.value !== undefined && !isNaN(p.value)) returnData2Map.set(p.time, p.value); });

        returnData1.forEach(p1 => {
            if (p1.value !== undefined && !isNaN(p1.value)) {
                const p2Value = returnData2Map.get(p1.time);
                if (p2Value !== undefined) {
                    pairedValues1.push(p1.value);
                    pairedValues2.push(p2Value);
                }
            }
        });

        if (pairedValues1.length > 1) {
            const correlation = calculateCorrelation(pairedValues1, pairedValues2);
            if (statsDisplayDiv && !isNaN(correlation)) {
                // Use seriesInfo.name for display
                const name1 = series1Info.name || series1Info.code;
                const name2 = series2Info.name || series2Info.code;
                statsDisplayDiv.innerHTML = `<strong>Correlation (${name1} vs ${name2} Returns):</strong> ${correlation.toFixed(4)}`;
            }
        } else {
            if (statsDisplayDiv) statsDisplayDiv.innerHTML = `<strong>Correlation:</strong> Not enough overlapping data.`;
        }
    }


    // --- Synchronize Charts ---
    if (priceChart && returnChart && firstSeriesId && activeSeries[firstSeriesId]) { // Ensure firstSeriesId is still valid
        priceChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (returnChart && returnChart.timeScale()) returnChart.timeScale().setVisibleLogicalRange(range); });
        returnChart.timeScale().subscribeVisibleLogicalRangeChange(range => { if (priceChart && priceChart.timeScale()) priceChart.timeScale().setVisibleLogicalRange(range); });
        let isSyncing = false;
        priceChart.subscribeCrosshairMove(param => {
            if (isSyncing || !returnChart || !param || !param.time || !activeSeries[firstSeriesId] || !activeSeries[firstSeriesId].seriesRefs.return) return;
            isSyncing = true; returnChart.moveCrosshair(param.point); isSyncing = false;
        });
        returnChart.subscribeCrosshairMove(param => {
            if (isSyncing || !priceChart || !param || !param.time || !activeSeries[firstSeriesId] || !activeSeries[firstSeriesId].seriesRefs.price) return;
            isSyncing = true; priceChart.moveCrosshair(param.point); isSyncing = false;
        });
    }

    if (priceChart) priceChart.timeScale().fitContent();
    if (returnChart) returnChart.timeScale().fitContent();

    updateActiveSeriesListUI();
    setLoading(false);
}

// --- UI Update Functions ---
function updateActiveSeriesListUI() {
    activeSeriesListDiv.innerHTML = '<strong>Active Series:</strong><br>';
    if (Object.keys(activeSeries).length === 0) {
        activeSeriesListDiv.innerHTML += '<i>None added yet.</i>'; return;
    }
    for (const id in activeSeries) {
        const seriesInfo = activeSeries[id];
        const displayName = seriesInfo.name || seriesInfo.code; // Use company name if available
        const itemDiv = document.createElement('div');
        itemDiv.classList.add('active-series-item');
        itemDiv.innerHTML = `<span style="background-color:${seriesInfo.color}; width: 12px; height: 12px; display: inline-block; margin-right: 8px; border: 1px solid #ccc;"></span> ${displayName} (${seriesInfo.type.toUpperCase()}) - Scale: ${seriesInfo.priceScaleId} <span class="remove-btn" data-id="${id}" title="Remove Series">✖</span>`;
        itemDiv.querySelector('.remove-btn').addEventListener('click', (e) => removeSeries(e.target.dataset.id));
        activeSeriesListDiv.appendChild(itemDiv);
    }
}

// --- Add/Remove Series Logic ---
async function addSeries() {
    const type = dataTypeSelect.value;
    const code = dataCodeInput.value.trim().toUpperCase();
    const startDate = startDateInput.value;
    const endDate = endDateInput.value;
    if (!code) { alert('Please enter a code/ID.'); return; }
    if (!startDate || !endDate) { alert('Please select a start and end date.'); return; }
    const seriesId = generateSeriesId(type, code);
    if (activeSeries[seriesId]) { alert(`${code} (${type}) is already added.`); return; }
    if (!seriesDefinitionMap[type]) { alert(`Data type '${type}' is not supported.`); return; }

    setLoading(true);
    try {
        const fetchedResult = await fetchSeriesData(type, code, startDate, endDate);
        if (fetchedResult.dataPoints.length === 0) {
            alert(`No data found for ${code} (${type}).`);
        } else {
            activeSeries[seriesId] = {
                type: type,
                code: code,
                name: fetchedResult.companyName, // Store the fetched company name
                data: fetchedResult.dataPoints,  // Store the actual data points
                color: getNextColor(),
                priceScaleId: getNextPriceScaleId(),
                seriesRefs: {}
            };
            dataCodeInput.value = '';
            renderCharts();
        }
    } catch (error) {
        console.error(`Error adding series ${seriesId}:`, error);
        alert(`Failed to add series ${code} (${type}): ${error.message}.`);
    } finally { setLoading(false); }
}

function removeSeries(seriesId) {
    if (activeSeries[seriesId]) { delete activeSeries[seriesId]; renderCharts(); }
}

// --- Event Listeners ---
addButton.addEventListener('click', addSeries);
reloadButton.addEventListener('click', () => {
    const currentStartDate = startDateInput.value;
    const currentEndDate = endDateInput.value;
    if (!currentStartDate || !currentEndDate) { alert("Ensure dates are selected."); return; }
    const reloadInfo = { ...activeSeries }; activeSeries = {}; colorIndex = 0; nextPriceScaleId = 'right';
    if (Object.keys(reloadInfo).length > 0) {
        setLoading(true);
        let promises = Object.values(reloadInfo).map(info =>
            fetchSeriesData(info.type, info.code, currentStartDate, currentEndDate)
                .then(data => ({ ...info, data, error: null }))
                .catch(error => ({ ...info, data: [], error }))
        );
        Promise.all(promises).then(results => {
            results.forEach(res => {
                if (res.error) console.error(`Error reloading ${res.id}:`, res.error);
                else if (res.data.length > 0) activeSeries[generateSeriesId(res.type, res.code)] = { ...res, color: getNextColor(), priceScaleId: getNextPriceScaleId(), seriesRefs: {} };
                else console.warn(`No data on reload for ${res.id}`);
            });
            renderCharts();
        }).finally(() => setLoading(false));
    } else renderCharts();
});

// --- Initial Load ---
updateActiveSeriesListUI();
renderCharts();

// --- Resize Handler ---
let resizeTimeout;
window.addEventListener('resize', () => {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
        if (priceChart) try { priceChart.resize(priceChartContainer.clientWidth, priceChartContainer.clientHeight); } catch (e) { console.error("Resize price chart error:", e); }
        if (returnChart) try { returnChart.resize(returnChartContainer.clientWidth, returnChartContainer.clientHeight); } catch (e) { console.error("Resize return chart error:", e); }
    }, 200);
});