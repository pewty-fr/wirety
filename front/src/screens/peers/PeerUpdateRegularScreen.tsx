import React, { useEffect, useState } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, HelperText, Switch, Text, ActivityIndicator } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateEndpoint } from '../../utils/validation';
import { Peer } from '../../types/api';

export const PeerUpdateRegularScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [peer, setPeer] = useState<Peer | null>(null);

  const [name, setName] = useState('');
  const [endpoint, setEndpoint] = useState('');
  const [isIsolated, setIsIsolated] = useState(false);
  const [fullEncapsulation, setFullEncapsulation] = useState(false);
  const [additionalIPs, setAdditionalIPs] = useState('');
  const [errors, setErrors] = useState<{ [key: string]: string }>({});

  useEffect(() => {
    loadPeer();
  }, [peerId]);

  const loadPeer = async () => {
    try {
      setLoading(true);
      const data = await api.getPeer(networkId, peerId);
      setPeer(data);
      // Prefill form
      setName(data.name);
      setEndpoint(data.endpoint || '');
      setIsIsolated(data.is_isolated);
      setFullEncapsulation(data.full_encapsulation);
      setAdditionalIPs(data.additional_allowed_ips ? data.additional_allowed_ips.join(', ') : '');
    } catch (e) {
      console.error('Failed to load peer', e);
      setErrors({ load: 'Failed to load peer' });
    } finally {
      setLoading(false);
    }
  };

  const validate = () => {
    const newErrors: { [key: string]: string } = {};
    if (!name.trim()) newErrors.name = 'Name is required';
    if (endpoint && !validateEndpoint(endpoint)) {
      newErrors.endpoint = 'Invalid endpoint format (expected IP:PORT)';
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async () => {
    if (!validate()) return;
    setSaving(true);
    try {
      const additional_allowed_ips = additionalIPs
        .split(',')
        .map(ip => ip.trim())
        .filter(ip => ip);

      await api.updatePeer(networkId, peerId, {
        name,
        endpoint: endpoint || undefined,
        is_isolated: isIsolated,
        full_encapsulation: fullEncapsulation,
        additional_allowed_ips: additional_allowed_ips.length ? additional_allowed_ips : undefined,
      });
      navigation.goBack();
    } catch (e) {
      console.error('Failed to update peer', e);
      setErrors({ submit: 'Failed to update peer' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (!peer) {
    return (
      <View style={styles.centered}>
        <Text>Peer not found</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <View style={styles.form}>
        <Title>Update Regular Peer</Title>
        {errors.load && <HelperText type="error">{errors.load}</HelperText>}

        <TextInput label="Name" value={name} onChangeText={setName} error={errors.name} />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

        <TextInput label="Endpoint (optional)" value={endpoint} onChangeText={setEndpoint} placeholder="1.2.3.4:51820" error={errors.endpoint} />
        {errors.endpoint && <HelperText type="error">{errors.endpoint}</HelperText>}

        <View style={styles.switchRow}>
          <Text>Isolated</Text>
          <Switch value={isIsolated} onValueChange={setIsIsolated} />
        </View>
        <HelperText type="info">Isolated peers cannot connect to other regular peers</HelperText>

        <View style={styles.switchRow}>
          <Text>Full Encapsulation</Text>
          <Switch value={fullEncapsulation} onValueChange={setFullEncapsulation} />
        </View>
        <HelperText type="info">Route all traffic (0.0.0.0/0) through jump server</HelperText>

        <TextInput label="Additional Allowed IPs (optional)" value={additionalIPs} onChangeText={setAdditionalIPs} placeholder="192.168.1.0/24, 10.0.0.0/8" multiline />
        <HelperText type="info">Comma-separated list of additional IP ranges this peer can route</HelperText>

        {errors.submit && <HelperText type="error">{errors.submit}</HelperText>}

        <FormButton title="Save Changes" onPress={handleSubmit} loading={saving} />
        <FormButton title="Cancel" onPress={() => navigation.goBack()} mode="outlined" />
      </View>
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fff' },
  form: { padding: 16 },
  centered: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  switchRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginVertical: 8 },
});

export default PeerUpdateRegularScreen;
