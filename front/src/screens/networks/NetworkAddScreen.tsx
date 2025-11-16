import React, { useState } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, Chip, HelperText, ActivityIndicator, Text } from 'react-native-paper';
import { useNavigation } from '@react-navigation/native';
import api from '../../services/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateCIDR, validateDomain, suggestCIDRs } from '../../utils/validation';

export const NetworkAddScreen = () => {
  const navigation = useNavigation();
  const [name, setName] = useState('');
  const [cidr, setCidr] = useState('');
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<{ [key: string]: string }>({});
  // suggestion inputs
  const [maxPeers, setMaxPeers] = useState('');
  const [baseCIDR, setBaseCIDR] = useState('');
  const [count, setCount] = useState('1');
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [suggestLoading, setSuggestLoading] = useState(false);

  const validate = () => {
    const newErrors: { [key: string]: string } = {};
    
    if (!name.trim()) newErrors.name = 'Name is required';
    if (!cidr.trim()) {
      newErrors.cidr = 'CIDR is required';
    } else if (!validateCIDR(cidr)) {
      newErrors.cidr = 'Invalid CIDR format';
    }
    if (!domain.trim()) {
      newErrors.domain = 'Domain is required';
    } else if (!validateDomain(domain)) {
      newErrors.domain = 'Invalid domain format';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async () => {
    if (!validate()) return;

    setLoading(true);
    try {
      await api.createNetwork({ name, cidr, domain });
      navigation.goBack();
    } catch (error) {
      console.error('Failed to create network:', error);
      setErrors({ submit: 'Failed to create network' });
    } finally {
      setLoading(false);
    }
  };

  const handleSuggest = async () => {
    setErrors((prev) => {
      const copy = { ...prev };
      delete copy.suggest;
      return copy;
    });
    const maxPeersNum = parseInt(maxPeers, 10);
    if (!maxPeers || isNaN(maxPeersNum) || maxPeersNum <= 0) {
      setErrors((prev) => ({ ...prev, max_peers: 'Max peers must be a positive integer' }));
      return;
    }
    const countNum = parseInt(count, 10);
    if (isNaN(countNum) || countNum <= 0) {
      setErrors((prev) => ({ ...prev, count: 'Count must be a positive integer' }));
      return;
    }
    setSuggestLoading(true);
    try {
      const data = await api.getAvailableCIDRs(maxPeersNum, countNum, baseCIDR || undefined);
      setSuggestions(data.cidrs);
    } catch (e) {
      console.error('Failed to fetch CIDR suggestions', e);
      setErrors((prev) => ({ ...prev, suggest: 'Failed to fetch suggestions' }));
    } finally {
      setSuggestLoading(false);
    }
  };

  return (
    <ScrollView style={styles.container}>
      <View style={styles.form}>
        <Title>Create Network</Title>
        
        <TextInput
          label="Name"
          value={name}
          onChangeText={setName}
          placeholder="My Network"
          error={errors.name}
        />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

        <TextInput
          label="CIDR (or choose a suggestion)"
          value={cidr}
          onChangeText={setCidr}
          placeholder="10.0.0.0/24"
          error={errors.cidr}
        />
        {errors.cidr && <HelperText type="error">{errors.cidr}</HelperText>}

        <Title style={styles.sectionTitle}>Suggest CIDRs</Title>
        <TextInput
          label="Max Peers (required)"
          value={maxPeers}
          onChangeText={setMaxPeers}
          placeholder="50"
          keyboardType="numeric"
          error={errors.max_peers}
        />
        {errors.max_peers && <HelperText type="error">{errors.max_peers}</HelperText>}
        <TextInput
          label="Base CIDR (optional)"
          value={baseCIDR}
          onChangeText={setBaseCIDR}
          placeholder="10.0.0.0/8"
        />
        <TextInput
          label="Count (optional)"
          value={count}
          onChangeText={setCount}
          placeholder="1"
          keyboardType="numeric"
          error={errors.count}
        />
        {errors.count && <HelperText type="error">{errors.count}</HelperText>}
        {errors.suggest && <HelperText type="error">{errors.suggest}</HelperText>}
        <FormButton title="Get Suggestions" onPress={handleSuggest} loading={suggestLoading} />

        <View style={styles.chipContainer}>
          {suggestLoading && <ActivityIndicator style={{ margin: 8 }} />}
          {!suggestLoading && suggestions.map((s) => (
            <Chip key={s} onPress={() => setCidr(s)} style={styles.chip}>{s}</Chip>
          ))}
          {!suggestLoading && suggestions.length === 0 && (
            <Text style={styles.hint}>Enter max peers then tap Get Suggestions</Text>
          )}
        </View>

        <Title style={styles.sectionTitle}>Quick Presets</Title>
        <View style={styles.chipContainer}>
          {suggestCIDRs().map((preset) => (
            <Chip key={preset} onPress={() => setCidr(preset)} style={styles.chip}>{preset}</Chip>
          ))}
        </View>

        <TextInput
          label="Domain"
          value={domain}
          onChangeText={setDomain}
          placeholder="vpn.example.com"
          error={errors.domain}
        />
        {errors.domain && <HelperText type="error">{errors.domain}</HelperText>}

        {errors.submit && <HelperText type="error">{errors.submit}</HelperText>}

        <FormButton
          title="Create Network"
          onPress={handleSubmit}
          loading={loading}
        />
        <FormButton
          title="Cancel"
          onPress={() => navigation.goBack()}
          mode="outlined"
        />
      </View>
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  form: {
    padding: 16,
  },
  chipContainer: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    marginVertical: 8,
  },
  chip: {
    margin: 4,
  },
  sectionTitle: {
    marginTop: 24,
    marginBottom: 8,
    fontSize: 16,
  },
  hint: {
    marginHorizontal: 8,
    color: '#666',
  },
});
