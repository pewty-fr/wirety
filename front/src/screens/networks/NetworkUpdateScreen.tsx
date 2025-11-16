import React, { useState, useEffect } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, HelperText } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { Network } from '../../types/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateDomain } from '../../utils/validation';

export const NetworkUpdateScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { id } = route.params as { id: string };
  const [name, setName] = useState('');
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<{ [key: string]: string }>({});

  useEffect(() => {
    loadNetwork();
  }, [id]);

  const loadNetwork = async () => {
    try {
      const network = await api.getNetwork(id);
      setName(network.name);
      setDomain(network.domain);
    } catch (error) {
      console.error('Failed to load network:', error);
    }
  };

  const validate = () => {
    const newErrors: { [key: string]: string } = {};
    
    if (!name.trim()) newErrors.name = 'Name is required';
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
      await api.updateNetwork(id, { name, domain });
      navigation.goBack();
    } catch (error) {
      console.error('Failed to update network:', error);
      setErrors({ submit: 'Failed to update network' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView style={styles.container}>
      <View style={styles.form}>
        <Title>Update Network</Title>
        
        <TextInput
          label="Name"
          value={name}
          onChangeText={setName}
          placeholder="My Network"
          error={errors.name}
        />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

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
          title="Update Network"
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
});
