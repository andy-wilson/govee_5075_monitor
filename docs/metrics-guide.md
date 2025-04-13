# Enhanced Environmental Metrics Guide

This guide provides detailed information about the additional environmental metrics available in the Govee Monitoring System.

## Available Metrics

The system now calculates the following metrics from the base temperature and humidity readings:

| Metric | Unit | Description |
|--------|------|-------------|
| Temperature | °C / °F | The ambient temperature |
| Humidity | % | Relative humidity |
| Absolute Humidity | g/m³ | Mass of water vapor per cubic meter of air |
| Dew Point | °C / °F | Temperature at which air becomes saturated with water vapor |
| Steam Pressure | hPa | Partial pressure of water vapor in the air |

## Understanding the Metrics

### Absolute Humidity

While relative humidity (%) tells you how saturated the air is with water vapor compared to its maximum capacity at the current temperature, absolute humidity tells you the actual mass of water vapor present in a given volume of air, regardless of temperature.

**Key points:**
- Measured in grams per cubic meter (g/m³)
- Not affected by temperature changes (unlike relative humidity)
- Useful for comparing actual moisture content between different environments
- Typically ranges from 0 g/m³ (completely dry) to 30+ g/m³ (extremely humid)

**Applications:**
- HVAC system optimization
- Industrial processes requiring specific moisture levels
- Material preservation (museums, archives)
- Indoor air quality monitoring

### Dew Point

The dew point is the temperature to which air must be cooled to reach saturation (100% relative humidity), causing condensation to form. It's a crucial metric for predicting when condensation might occur on surfaces.

**Key points:**
- Measured in degrees Celsius (°C) or Fahrenheit (°F)
- When air temperature equals the dew point, condensation forms
- A higher dew point feels more uncomfortable to people
- The closer the dew point is to the air temperature, the higher the relative humidity

**Applications:**
- Predicting fog and dew formation
- Preventing condensation on surfaces
- Mold prevention in buildings
- Comfort analysis for indoor environments

### Steam Pressure

Also called vapor pressure, this is the partial pressure exerted by water vapor in the air.

**Key points:**
- Measured in hectopascals (hPa)
- Indicates the pressure exerted by water molecules in the air
- Increases with temperature at a given relative humidity
- Directly related to evaporation and condensation rates

**Applications:**
- Plant cultivation environments
- Weather prediction
- Industrial processes involving evaporation
- Scientific research requiring precise environmental control

## Calculation Methods

### Absolute Humidity

Calculated using the formula:
```
absHumidity = 216.7 * (relHumidity/100 * 6.112 * exp(17.62*tempC/(243.12+tempC)) / (273.15+tempC))
```

This formula:
1. Calculates saturation vapor pressure using the Magnus equation
2. Determines actual vapor pressure using relative humidity
3. Converts to absolute humidity using gas laws

### Dew Point

Calculated using the Magnus formula:
```
dewPoint = 243.12 * ln(relHumidity/100 * exp(17.62*tempC/(243.12+tempC))) / (17.62 - ln(relHumidity/100 * exp(17.62*tempC/(243.12+tempC))))
```

This is a widely-accepted approximation that works well in normal atmospheric conditions and temperature ranges.

### Steam Pressure

Calculated using:
```
steamPressure = relHumidity/100 * 6.112 * exp(17.62*tempC/(243.12+tempC))
```

This is directly related to the saturation vapor pressure, adjusted for the actual relative humidity.

## Sensor Calibration

The system supports calibration adjustments to improve accuracy:

### Temperature Offset

Applied directly to raw temperature readings:
```
adjustedTemp = rawTemp + tempOffset
```

Use the `-temp-offset` flag to set this value.

### Humidity Offset

Applied directly to raw humidity readings:
```
adjustedHumidity = rawHumidity + humidityOffset
```

Use the `-humidity-offset` flag to set this value.

## Setting Calibration Values

To determine appropriate calibration values:

1. Place the Govee sensor near a trusted reference instrument
2. Allow both to stabilize for at least 30 minutes
3. Note the readings from both devices
4. Calculate the offsets:
   - Temperature offset = reference temperature - Govee temperature
   - Humidity offset = reference humidity - Govee humidity
5. Use these values with the client flags:

```bash
./govee-client -temp-offset=1.2 -humidity-offset=-3.5
```

## Practical Applications

### Home Environment Monitoring

- **Mold Prevention**: Keep dew point well below surface temperatures (especially on exterior walls)
- **HVAC Efficiency**: Optimize heating/cooling based on absolute humidity
- **Comfort Monitoring**: Track both temperature and humidity for better comfort assessment

### Industrial Applications

- **Material Storage**: Monitor dew point to prevent condensation on sensitive materials
- **Process Control**: Use absolute humidity for consistent environmental conditions
- **Equipment Protection**: Prevent condensation on electronic equipment

### Scientific and Conservation Use

- **Museum Collections**: Monitor all metrics for optimal preservation conditions
- **Laboratory Environments**: Maintain precise environmental control
- **Plant Growing**: Optimize conditions based on absolute humidity and vapor pressure

## Visualizing the Enhanced Metrics

The system's dashboard displays all enhanced metrics:

- **Current Values**: Each metric is shown with its current value
- **Historical Charts**: Time-series charts for each metric
- **Calibration Indicators**: Visual indicators show when calibration is applied

You can use time-range selections to analyze how these metrics change over different periods, helping to identify patterns and potential issues.

## Example Output

The client's console output now includes the enhanced metrics:

```
2023-04-10T15:30:45 GVH5075_1234 Temp: 22.5°C/72.5°F, Humidity: 45.5%, Dew Point: 10.2°C, AH: 9.1 g/m³, SP: 12.3 hPa, Battery: 87%, RSSI: -67dBm
```

This comprehensive output gives you a complete picture of the environmental conditions at a glance.
