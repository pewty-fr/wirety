import React, { useEffect, useState } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, HelperText, ActivityIndicator, Text } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateEndpoint, validatePort } from '../../utils/validation';
import { Peer } from '../../types/api';

export const PeerUpdateJumpScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId, peerId } = route.params as { networkId: string; peerId: string };
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [peer, setPeer] = useState<Peer | null>(null);

  const [name, setName] = useState('');
  const [endpoint, setEndpoint] = useState('');
  const [listenPort, setListenPort] = useState('');
  const [natInterface, setNatInterface] = useState(''); // read-only, can't update currently
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
      setName(data.name);
      setEndpoint(data.endpoint || '');
      setListenPort(data.listen_port ? String(data.listen_port) : '51820');
      setNatInterface(data.jump_nat_interface || '');
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
    if (!endpoint.trim()) {
      newErrors.endpoint = 'Endpoint is required';
    } else if (!validateEndpoint(endpoint)) {
      newErrors.endpoint = 'Invalid endpoint format (expected IP:PORT)';
    }
    const portNum = parseInt(listenPort, 10);
    if (!validatePort(portNum)) newErrors.listenPort = 'Invalid port number';
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
        endpoint,
        listen_port: parseInt(listenPort, 10),
        additional_allowed_ips: additional_allowed_ips.length ? additional_allowed_ips : undefined,
      });
      navigation.goBack();
    } catch (e) {
      console.error('Failed to update jump server', e);
      setErrors({ submit: 'Failed to update jump server' });
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
        <Title>Update Jump Server</Title>
        {errors.load && <HelperText type="error">{errors.load}</HelperText>}

        <TextInput label="Name" value={name} onChangeText={setName} error={errors.name} />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

        <TextInput label="Endpoint" value={endpoint} onChangeText={setEndpoint} placeholder="1.2.3.4:51820" error={errors.endpoint} />
        {errors.endpoint && <HelperText type="error">{errors.endpoint}</HelperText>}

        <TextInput label="Listen Port" value={listenPort} onChangeText={setListenPort} keyboardType="numeric" placeholder="51820" error={errors.listenPort} />
        {errors.listenPort && <HelperText type="error">{errors.listenPort}</HelperText>}

        <TextInput label="NAT Interface (read-only)" value={natInterface} onChangeText={() => {}} disabled editable={false} />
        <HelperText type="info">NAT interface cannot be changed after creation (backend limitation)</HelperText>

        <TextInput label="Additional Allowed IPs (optional)" value={additionalIPs} onChangeText={setAdditionalIPs} multiline placeholder="192.168.1.0/24, 10.0.0.0/8" />
        <HelperText type="info">Comma-separated list of additional IP ranges this jump server can route</HelperText>

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
});

export default PeerUpdateJumpScreen;
